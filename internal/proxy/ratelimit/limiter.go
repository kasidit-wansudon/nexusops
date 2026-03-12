package ratelimit

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// bucket is a token bucket that refills at a steady rate over time.
type bucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64   // tokens per second
	lastRefill time.Time // last time tokens were refilled
}

// BucketStats exposes the current state of a rate limit bucket.
type BucketStats struct {
	Remaining float64   `json:"remaining"`
	Limit     float64   `json:"limit"`
	ResetAt   time.Time `json:"reset_at"`
}

// Limiter implements per-key token bucket rate limiting. Each key (e.g., an IP
// address or route) gets its own bucket with configurable rate and burst.
type Limiter struct {
	buckets      map[string]*bucket
	mu           sync.RWMutex
	defaultRate  float64 // tokens per second
	defaultBurst int     // max tokens (burst capacity)
}

// NewLimiter creates a new rate limiter with the given default refill rate
// (tokens per second) and burst size (maximum tokens per bucket).
func NewLimiter(defaultRate float64, defaultBurst int) *Limiter {
	if defaultRate <= 0 {
		defaultRate = 10.0
	}
	if defaultBurst <= 0 {
		defaultBurst = 20
	}
	return &Limiter{
		buckets:      make(map[string]*bucket),
		defaultRate:  defaultRate,
		defaultBurst: defaultBurst,
	}
}

// getOrCreate returns the bucket for the given key, creating one with default
// settings if it does not exist.
func (l *Limiter) getOrCreate(key string) *bucket {
	b, exists := l.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     float64(l.defaultBurst),
			maxTokens:  float64(l.defaultBurst),
			refillRate: l.defaultRate,
			lastRefill: time.Now(),
		}
		l.buckets[key] = b
	}
	return b
}

// refill adds tokens to the bucket based on elapsed time since the last refill.
func (b *bucket) refill(now time.Time) {
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
}

// Allow checks whether the given key has available tokens. It refills the
// bucket based on elapsed time, then attempts to consume one token. Returns
// true if the request is allowed, false if rate limited.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.getOrCreate(key)
	now := time.Now()
	b.refill(now)

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}

// SetRate configures a custom rate and burst for a specific key. This creates
// the bucket if it does not exist, or updates the existing bucket's parameters.
func (l *Limiter) SetRate(key string, rate float64, burst int) {
	if rate <= 0 {
		rate = l.defaultRate
	}
	if burst <= 0 {
		burst = l.defaultBurst
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists := l.buckets[key]
	if !exists {
		l.buckets[key] = &bucket{
			tokens:     float64(burst),
			maxTokens:  float64(burst),
			refillRate: rate,
			lastRefill: time.Now(),
		}
		return
	}

	b.refillRate = rate
	b.maxTokens = float64(burst)
	// Cap current tokens to new max
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
}

// Reset removes the bucket for a specific key, resetting its rate limit state.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// Middleware returns an HTTP middleware that applies rate limiting to incoming
// requests. The keyFunc extracts the rate-limit key from each request (e.g.,
// client IP, host). Requests that exceed the rate limit receive a 429 response.
func (l *Limiter) Middleware(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" {
				// If no key can be extracted, allow the request
				next.ServeHTTP(w, r)
				return
			}

			if !l.Allow(key) {
				stats := l.GetStats(key)

				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", stats.Limit))
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%.0f", stats.Remaining))
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", stats.ResetAt.Unix()))
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", time.Until(stats.ResetAt).Seconds()))
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")

				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Set rate limit headers on successful requests too
			stats := l.GetStats(key)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", stats.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%.0f", stats.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", stats.ResetAt.Unix()))

			next.ServeHTTP(w, r)
		})
	}
}

// IPKeyFunc extracts the client IP from a request for use as a rate limit key.
// It checks X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr.
func IPKeyFunc(r *http.Request) string {
	// Check X-Forwarded-For first (may be set by proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain (original client)
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr, stripping the port
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return r.RemoteAddr
	}
	return host
}

// RouteKeyFunc uses the request's Host header as the rate limit key,
// applying rate limits per route/subdomain.
func RouteKeyFunc(r *http.Request) string {
	host := r.Host
	// Strip port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}

// Cleanup removes buckets that have not been accessed for longer than maxAge.
// This prevents unbounded memory growth from one-off clients. It should be
// called periodically (e.g., every few minutes).
func (l *Limiter) Cleanup(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for key, b := range l.buckets {
		if b.lastRefill.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}

// GetStats returns the current rate limit statistics for a given key.
// If the key has no bucket, stats reflect the default configuration.
func (l *Limiter) GetStats(key string) *BucketStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	b, exists := l.buckets[key]
	if !exists {
		return &BucketStats{
			Remaining: float64(l.defaultBurst),
			Limit:     float64(l.defaultBurst),
			ResetAt:   time.Now(),
		}
	}

	// Calculate how long until the bucket is full
	remaining := math.Max(0, b.tokens)
	tokensNeeded := b.maxTokens - remaining
	var resetAt time.Time
	if tokensNeeded <= 0 || b.refillRate <= 0 {
		resetAt = time.Now()
	} else {
		secondsToFull := tokensNeeded / b.refillRate
		resetAt = time.Now().Add(time.Duration(secondsToFull * float64(time.Second)))
	}

	return &BucketStats{
		Remaining: remaining,
		Limit:     b.maxTokens,
		ResetAt:   resetAt,
	}
}

// BucketCount returns the number of active buckets, useful for monitoring.
func (l *Limiter) BucketCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.buckets)
}
