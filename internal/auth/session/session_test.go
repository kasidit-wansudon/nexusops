package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSession(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	sess, err := store.Create("user-1", map[string]string{"ip": "127.0.0.1"})
	require.NoError(t, err)

	assert.NotEmpty(t, sess.ID)
	assert.Equal(t, "user-1", sess.UserID)
	assert.NotEmpty(t, sess.Token, "token should be returned on creation")
	assert.False(t, sess.CreatedAt.IsZero())
	assert.False(t, sess.ExpiresAt.IsZero())
	assert.True(t, sess.ExpiresAt.After(sess.CreatedAt))
	assert.Equal(t, "127.0.0.1", sess.Metadata["ip"])
}

func TestCreateSessionEmptyUserID(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	_, err := store.Create("", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID must not be empty")
}

func TestCreateSessionUniqueTokens(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	sess1, err := store.Create("user-1", nil)
	require.NoError(t, err)
	sess2, err := store.Create("user-1", nil)
	require.NoError(t, err)

	assert.NotEqual(t, sess1.Token, sess2.Token, "each session should have a unique token")
	assert.NotEqual(t, sess1.ID, sess2.ID, "each session should have a unique ID")
}

func TestGetSession(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	created, err := store.Create("user-1", map[string]string{"browser": "chrome"})
	require.NoError(t, err)

	retrieved, err := store.Get(created.Token)
	require.NoError(t, err)

	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.UserID, retrieved.UserID)
	assert.Equal(t, "chrome", retrieved.Metadata["browser"])
}

func TestGetSessionNotFound(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	_, err := store.Get("nonexistent-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSessionEmptyToken(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	_, err := store.Get("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token must not be empty")
}

func TestDeleteSession(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	sess, err := store.Create("user-1", nil)
	require.NoError(t, err)

	// Should retrieve before deletion.
	_, err = store.Get(sess.Token)
	require.NoError(t, err)

	// Delete the session.
	err = store.Delete(sess.Token)
	require.NoError(t, err)

	// Should not find after deletion.
	_, err = store.Get(sess.Token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteSessionIdempotent(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	// Deleting a non-existent token should not error.
	err := store.Delete("does-not-exist")
	assert.NoError(t, err)
}

func TestSessionExpiry(t *testing.T) {
	// Use a very short TTL.
	store := NewMemoryStore(1 * time.Millisecond)

	sess, err := store.Create("user-1", nil)
	require.NoError(t, err)

	// Wait for it to expire.
	time.Sleep(10 * time.Millisecond)

	_, err = store.Get(sess.Token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestSessionIsExpired(t *testing.T) {
	sess := &Session{
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	assert.True(t, sess.IsExpired())

	sess.ExpiresAt = time.Now().UTC().Add(1 * time.Hour)
	assert.False(t, sess.IsExpired())
}

func TestSessionCleanup(t *testing.T) {
	store := NewMemoryStore(1 * time.Millisecond)

	_, err := store.Create("user-1", nil)
	require.NoError(t, err)
	_, err = store.Create("user-2", nil)
	require.NoError(t, err)
	_, err = store.Create("user-3", nil)
	require.NoError(t, err)

	// Wait for sessions to expire.
	time.Sleep(10 * time.Millisecond)

	removed := store.Cleanup()
	assert.Equal(t, 3, removed, "all 3 expired sessions should be removed")

	// Create a fresh session that is still active.
	store2 := NewMemoryStore(1 * time.Hour)
	_, err = store2.Create("user-alive", nil)
	require.NoError(t, err)

	removed = store2.Cleanup()
	assert.Equal(t, 0, removed, "no active sessions should be removed")
	assert.Equal(t, 1, store2.ActiveCount())
}

func TestSessionMetadata(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	meta := map[string]string{
		"ip":         "10.0.0.1",
		"user_agent": "TestAgent/1.0",
		"device":     "mobile",
	}

	sess, err := store.Create("user-1", meta)
	require.NoError(t, err)

	assert.Equal(t, "10.0.0.1", sess.Metadata["ip"])
	assert.Equal(t, "TestAgent/1.0", sess.Metadata["user_agent"])
	assert.Equal(t, "mobile", sess.Metadata["device"])

	// Verify that mutating the original map does not affect the stored session.
	meta["ip"] = "modified"
	retrieved, err := store.Get(sess.Token)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", retrieved.Metadata["ip"], "metadata should be a defensive copy")
}

func TestSessionMetadataNil(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	sess, err := store.Create("user-1", nil)
	require.NoError(t, err)

	assert.NotNil(t, sess.Metadata, "metadata should be initialized even when nil is passed")
	assert.Empty(t, sess.Metadata)
}

func TestActiveCount(t *testing.T) {
	store := NewMemoryStore(1 * time.Hour)

	assert.Equal(t, 0, store.ActiveCount())

	_, err := store.Create("user-1", nil)
	require.NoError(t, err)
	_, err = store.Create("user-2", nil)
	require.NoError(t, err)

	assert.Equal(t, 2, store.ActiveCount())
}

func TestDefaultTTL(t *testing.T) {
	// Passing zero TTL should default to 24 hours.
	store := NewMemoryStore(0)

	sess, err := store.Create("user-1", nil)
	require.NoError(t, err)

	expectedExpiry := sess.CreatedAt.Add(24 * time.Hour)
	diff := sess.ExpiresAt.Sub(expectedExpiry)
	assert.True(t, diff < time.Second && diff > -time.Second, "default TTL should be 24 hours")
}
