package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewMemoryStoreDefaultTTL(t *testing.T) {
	store := NewMemoryStore(0)
	require.NotNil(t, store)
	assert.Equal(t, 24*time.Hour, store.ttl)
}

func TestNewMemoryStoreCustomTTL(t *testing.T) {
	store := NewMemoryStore(2 * time.Hour)
	assert.Equal(t, 2*time.Hour, store.ttl)
}

func TestCreateSession(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	sess, err := store.Create("user1", map[string]string{"ip": "1.2.3.4"})
	require.NoError(t, err)

	assert.Equal(t, "user1", sess.UserID)
	assert.NotEmpty(t, sess.Token)
	assert.Equal(t, "sess_1", sess.ID)
	assert.Equal(t, "1.2.3.4", sess.Metadata["ip"])
	assert.False(t, sess.IsExpired())
}

func TestCreateSessionEmptyUserID(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	_, err := store.Create("", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user ID must not be empty")
}

func TestGetSession(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	created, _ := store.Create("user1", nil)

	fetched, err := store.Get(created.Token)
	require.NoError(t, err)
	assert.Equal(t, created.UserID, fetched.UserID)
	assert.Equal(t, created.ID, fetched.ID)
}

func TestGetSessionNotFound(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	_, err := store.Get("nonexistent-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSessionEmptyToken(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	_, err := store.Get("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token must not be empty")
}

func TestGetSessionExpired(t *testing.T) {
	store := NewMemoryStore(1 * time.Millisecond)
	sess, _ := store.Create("user1", nil)
	time.Sleep(5 * time.Millisecond)

	_, err := store.Get(sess.Token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestDeleteSession(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	sess, _ := store.Create("user1", nil)

	err := store.Delete(sess.Token)
	require.NoError(t, err)

	_, err = store.Get(sess.Token)
	require.Error(t, err)
}

func TestCleanup(t *testing.T) {
	store := NewMemoryStore(1 * time.Millisecond)
	store.Create("user1", nil)
	store.Create("user2", nil)
	time.Sleep(5 * time.Millisecond)

	removed := store.Cleanup()
	assert.Equal(t, 2, removed)
	assert.Equal(t, 0, store.ActiveCount())
}

func TestActiveCount(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	assert.Equal(t, 0, store.ActiveCount())

	store.Create("user1", nil)
	store.Create("user2", nil)
	assert.Equal(t, 2, store.ActiveCount())
}

func TestStartCleanupTicker(t *testing.T) {
	store := NewMemoryStore(1 * time.Millisecond)
	store.Create("user1", nil)

	stop := store.StartCleanupTicker(10 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	stop()

	assert.Equal(t, 0, store.ActiveCount())
}

func TestSessionIsExpired(t *testing.T) {
	s := &Session{ExpiresAt: time.Now().UTC().Add(-time.Hour)}
	assert.True(t, s.IsExpired())

	s2 := &Session{ExpiresAt: time.Now().UTC().Add(time.Hour)}
	assert.False(t, s2.IsExpired())
}

func TestAuthMiddlewareSuccess(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	sess, _ := store.Create("user1", nil)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	r.Use(AuthMiddleware(store))
	r.GET("/test", func(c *gin.Context) {
		uid := UserIDFromContext(c)
		s := SessionFromContext(c)
		c.JSON(http.StatusOK, gin.H{"user_id": uid, "session_id": s.ID})
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+sess.Token)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "user1")
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	store := NewMemoryStore(time.Hour)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(AuthMiddleware(store))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddlewareInvalidFormat(t *testing.T) {
	store := NewMemoryStore(time.Hour)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(AuthMiddleware(store))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic abc123")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddlewareEmptyBearer(t *testing.T) {
	store := NewMemoryStore(time.Hour)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(AuthMiddleware(store))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	store := NewMemoryStore(time.Hour)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(AuthMiddleware(store))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserIDFromContextEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Equal(t, "", UserIDFromContext(c))
}

func TestSessionFromContextNil(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Nil(t, SessionFromContext(c))
}

func TestCreateSessionMetadataCopied(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	meta := map[string]string{"key": "original"}
	sess, _ := store.Create("user1", meta)

	// Mutating the original map should not affect the stored session.
	meta["key"] = "mutated"
	fetched, _ := store.Get(sess.Token)
	assert.Equal(t, "original", fetched.Metadata["key"])
}
