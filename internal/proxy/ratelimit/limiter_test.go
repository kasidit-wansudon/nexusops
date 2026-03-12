package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowConsumesTokens(t *testing.T) {
	l := NewLimiter(100, 3) // 100 tokens/sec refill, burst of 3

	assert.True(t, l.Allow("user1"))
	assert.True(t, l.Allow("user1"))
	assert.True(t, l.Allow("user1"))
	// Bucket exhausted — should be denied immediately (no time to refill).
	assert.False(t, l.Allow("user1"))
}

func TestAllowDifferentKeys(t *testing.T) {
	l := NewLimiter(100, 1)

	assert.True(t, l.Allow("a"))
	assert.True(t, l.Allow("b"))
	// "a" is exhausted, but "b" was independent
	assert.False(t, l.Allow("a"))
	assert.False(t, l.Allow("b"))
}

func TestDefaultRateAndBurst(t *testing.T) {
	l := NewLimiter(0, 0)
	// Should default to rate=10.0, burst=20
	stats := l.GetStats("unknown-key")
	assert.Equal(t, float64(20), stats.Limit)
	assert.Equal(t, float64(20), stats.Remaining)
}

func TestSetRate(t *testing.T) {
	l := NewLimiter(10, 10)
	l.SetRate("custom", 50, 5)

	stats := l.GetStats("custom")
	assert.Equal(t, float64(5), stats.Limit)
}

func TestResetKey(t *testing.T) {
	l := NewLimiter(10, 2)
	l.Allow("user1")
	assert.Equal(t, 1, l.BucketCount())

	l.Reset("user1")
	assert.Equal(t, 0, l.BucketCount())
}

func TestCleanup(t *testing.T) {
	l := NewLimiter(10, 5)
	l.Allow("old")
	// Artificially age the bucket by setting lastRefill in the past.
	l.buckets["old"].lastRefill = time.Now().Add(-2 * time.Hour)

	l.Allow("new")

	l.Cleanup(1 * time.Hour)
	assert.Equal(t, 1, l.BucketCount())
	// "new" should still exist; "old" should be cleaned up.
	stats := l.GetStats("new")
	assert.Less(t, stats.Remaining, float64(5))
}

func TestBucketCount(t *testing.T) {
	l := NewLimiter(10, 5)
	assert.Equal(t, 0, l.BucketCount())

	l.Allow("a")
	l.Allow("b")
	l.Allow("c")
	assert.Equal(t, 3, l.BucketCount())
}

func TestMiddlewareAllows(t *testing.T) {
	l := NewLimiter(100, 5)

	handler := l.Middleware(IPKeyFunc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
}

func TestMiddlewareRateLimits(t *testing.T) {
	l := NewLimiter(100, 1) // burst of 1

	handler := l.Middleware(IPKeyFunc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	// First request: allowed
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second request: rate limited
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "rate limit exceeded")
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestIPKeyFunc(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(r *http.Request)
		expected string
	}{
		{
			"X-Forwarded-For",
			func(r *http.Request) { r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8") },
			"1.2.3.4",
		},
		{
			"X-Real-IP",
			func(r *http.Request) { r.Header.Set("X-Real-IP", "9.8.7.6") },
			"9.8.7.6",
		},
		{
			"RemoteAddr",
			func(r *http.Request) { r.RemoteAddr = "192.168.1.1:54321" },
			"192.168.1.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.setup(req)
			result := IPKeyFunc(req)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRouteKeyFunc(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "MyApp.Example.COM:8080"
	result := RouteKeyFunc(req)
	assert.Equal(t, "myapp.example.com", result)
}

func TestGetStatsForExistingBucket(t *testing.T) {
	l := NewLimiter(10, 5)
	l.Allow("key1")

	stats := l.GetStats("key1")
	require.NotNil(t, stats)
	assert.Equal(t, float64(5), stats.Limit)
	assert.Less(t, stats.Remaining, float64(5))
}
