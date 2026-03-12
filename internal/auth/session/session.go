// Package session provides session management with an in-memory store
// and Gin middleware for the NexusOps platform.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// tokenByteLength is the number of random bytes used to generate a session token.
	tokenByteLength = 32
	// contextKeyUserID is the Gin context key where the authenticated user ID is stored.
	contextKeyUserID = "userID"
	// contextKeySession is the Gin context key where the full session is stored.
	contextKeySession = "session"
)

// Session represents an authenticated user session.
type Session struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	Token     string            `json:"token,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// IsExpired reports whether the session has passed its expiration time.
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// Store defines the interface for session persistence.
type Store interface {
	// Create creates a new session for the given user.
	Create(userID string, metadata map[string]string) (*Session, error)
	// Get retrieves a session by its token, returning an error if not found or expired.
	Get(token string) (*Session, error)
	// Delete removes a session by its token.
	Delete(token string) error
	// Cleanup removes all expired sessions from the store.
	Cleanup() int
}

// MemoryStore is an in-memory implementation of Store protected by a
// read-write mutex. It is suitable for single-process deployments and
// testing.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session // token -> Session
	ttl      time.Duration
	counter  uint64 // monotonic session ID counter
}

// NewMemoryStore creates a new in-memory session store with the given
// default session TTL.
func NewMemoryStore(ttl time.Duration) *MemoryStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &MemoryStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

// Create generates a new session with a cryptographically random token.
func (ms *MemoryStore) Create(userID string, metadata map[string]string) (*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("session: user ID must not be empty")
	}

	tokenBytes := make([]byte, tokenByteLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("session: failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	now := time.Now().UTC()

	ms.mu.Lock()
	ms.counter++
	id := fmt.Sprintf("sess_%d", ms.counter)
	ms.mu.Unlock()

	// Copy metadata so the caller cannot mutate internal state.
	meta := make(map[string]string, len(metadata))
	for k, v := range metadata {
		meta[k] = v
	}

	session := &Session{
		ID:        id,
		UserID:    userID,
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(ms.ttl),
		Metadata:  meta,
	}

	ms.mu.Lock()
	ms.sessions[token] = session
	ms.mu.Unlock()

	// Return a copy with the token so the caller can send it to the client.
	result := *session
	return &result, nil
}

// Get retrieves a session by token. It returns an error if the session
// does not exist or has expired.
func (ms *MemoryStore) Get(token string) (*Session, error) {
	if token == "" {
		return nil, fmt.Errorf("session: token must not be empty")
	}

	ms.mu.RLock()
	session, ok := ms.sessions[token]
	ms.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session: not found")
	}

	if session.IsExpired() {
		// Lazy cleanup: remove expired session on access.
		ms.mu.Lock()
		delete(ms.sessions, token)
		ms.mu.Unlock()
		return nil, fmt.Errorf("session: expired")
	}

	result := *session
	return &result, nil
}

// Delete removes a session by its token. It is a no-op if the session
// does not exist.
func (ms *MemoryStore) Delete(token string) error {
	ms.mu.Lock()
	delete(ms.sessions, token)
	ms.mu.Unlock()
	return nil
}

// Cleanup removes all expired sessions and returns the number removed.
func (ms *MemoryStore) Cleanup() int {
	now := time.Now().UTC()
	removed := 0

	ms.mu.Lock()
	defer ms.mu.Unlock()

	for token, session := range ms.sessions {
		if now.After(session.ExpiresAt) {
			delete(ms.sessions, token)
			removed++
		}
	}

	return removed
}

// ActiveCount returns the number of non-expired sessions in the store.
func (ms *MemoryStore) ActiveCount() int {
	now := time.Now().UTC()
	count := 0

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	for _, session := range ms.sessions {
		if !now.After(session.ExpiresAt) {
			count++
		}
	}
	return count
}

// StartCleanupTicker starts a background goroutine that runs Cleanup at
// the given interval. It returns a stop function that halts the ticker.
func (ms *MemoryStore) StartCleanupTicker(interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				ms.Cleanup()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}

// AuthMiddleware returns a Gin middleware that validates session tokens
// from the Authorization header. It expects a "Bearer <token>" format.
// On success it sets "userID" and "session" in the Gin context. On
// failure it aborts with 401 Unauthorized.
func AuthMiddleware(store Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing Authorization header",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid Authorization header format, expected 'Bearer <token>'",
			})
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "empty bearer token",
			})
			return
		}

		sess, err := store.Get(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": fmt.Sprintf("invalid session: %v", err),
			})
			return
		}

		c.Set(contextKeyUserID, sess.UserID)
		c.Set(contextKeySession, sess)
		c.Next()
	}
}

// UserIDFromContext extracts the authenticated user ID from a Gin context.
// Returns an empty string if the middleware has not run or the user is
// not authenticated.
func UserIDFromContext(c *gin.Context) string {
	val, exists := c.Get(contextKeyUserID)
	if !exists {
		return ""
	}
	uid, ok := val.(string)
	if !ok {
		return ""
	}
	return uid
}

// SessionFromContext extracts the full Session from a Gin context.
// Returns nil if the middleware has not run.
func SessionFromContext(c *gin.Context) *Session {
	val, exists := c.Get(contextKeySession)
	if !exists {
		return nil
	}
	sess, ok := val.(*Session)
	if !ok {
		return nil
	}
	return sess
}
