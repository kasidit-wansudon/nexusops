package apikey

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager() *Manager {
	return NewManager([]byte("test-encryption-key-32bytes!!!!"))
}

func TestGenerate(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("my-key", "proj-1", "user-1", []string{"read", "write"})
	require.NoError(t, err)

	assert.NotEmpty(t, key.ID)
	assert.NotEmpty(t, key.Key)
	assert.True(t, strings.HasPrefix(key.Key, "nxo_"), "key should start with nxo_ prefix, got: %s", key.Key)
	assert.Equal(t, "my-key", key.Name)
	assert.Equal(t, "proj-1", key.ProjectID)
	assert.Equal(t, "user-1", key.UserID)
	assert.Equal(t, []string{"read", "write"}, key.Permissions)
	assert.False(t, key.Revoked)
	assert.False(t, key.CreatedAt.IsZero())
	assert.False(t, key.ExpiresAt.IsZero())
	assert.True(t, key.ExpiresAt.After(key.CreatedAt))

	// Key should be nxo_ + 64 hex chars (32 bytes)
	rawHex := strings.TrimPrefix(key.Key, "nxo_")
	assert.Len(t, rawHex, 64, "hex portion should be 64 characters")
}

func TestGenerateEmptyName(t *testing.T) {
	mgr := newTestManager()

	_, err := mgr.Generate("", "proj-1", "user-1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

func TestGenerateEmptyUserID(t *testing.T) {
	mgr := newTestManager()

	_, err := mgr.Generate("key-name", "proj-1", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID must not be empty")
}

func TestValidate(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("my-key", "proj-1", "user-1", []string{"read"})
	require.NoError(t, err)

	validated, err := mgr.Validate(key.Key)
	require.NoError(t, err)
	assert.Equal(t, key.ID, validated.ID)
	assert.Equal(t, "my-key", validated.Name)
	assert.Equal(t, "proj-1", validated.ProjectID)
	assert.Equal(t, "user-1", validated.UserID)
	assert.Empty(t, validated.Key, "plaintext key should not be returned on validate")
	assert.False(t, validated.LastUsedAt.IsZero(), "LastUsedAt should be updated after validation")
}

func TestValidateExpired(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("expired-key", "proj-1", "user-1", nil)
	require.NoError(t, err)

	// Manually set the expiry in the past.
	mgr.mu.Lock()
	stored := mgr.keys[key.ID]
	stored.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
	mgr.mu.Unlock()

	_, err = mgr.Validate(key.Key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestValidateRevoked(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("revoked-key", "proj-1", "user-1", nil)
	require.NoError(t, err)

	err = mgr.Revoke(key.ID)
	require.NoError(t, err)

	_, err = mgr.Validate(key.Key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRevoke(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("revoke-me", "proj-1", "user-1", nil)
	require.NoError(t, err)

	// Should validate before revoke.
	_, err = mgr.Validate(key.Key)
	require.NoError(t, err)

	// Revoke the key.
	err = mgr.Revoke(key.ID)
	require.NoError(t, err)

	// Should fail to validate after revoke.
	_, err = mgr.Validate(key.Key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRevokeNonexistent(t *testing.T) {
	mgr := newTestManager()

	err := mgr.Revoke("nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestList(t *testing.T) {
	mgr := newTestManager()

	// Generate keys for two users.
	_, err := mgr.Generate("key-1", "proj-1", "user-1", nil)
	require.NoError(t, err)
	_, err = mgr.Generate("key-2", "proj-1", "user-1", nil)
	require.NoError(t, err)
	_, err = mgr.Generate("key-3", "proj-2", "user-2", nil)
	require.NoError(t, err)

	// List for user-1 should return 2 keys.
	keys := mgr.List("user-1")
	assert.Len(t, keys, 2)
	for _, k := range keys {
		assert.Equal(t, "user-1", k.UserID)
		assert.Empty(t, k.Key, "plaintext key should not be in list results")
	}

	// List for user-2 should return 1 key.
	keys = mgr.List("user-2")
	assert.Len(t, keys, 1)

	// List for unknown user should return empty.
	keys = mgr.List("user-unknown")
	assert.Empty(t, keys)
}

func TestListExcludesRevoked(t *testing.T) {
	mgr := newTestManager()

	key1, err := mgr.Generate("key-1", "proj-1", "user-1", nil)
	require.NoError(t, err)
	_, err = mgr.Generate("key-2", "proj-1", "user-1", nil)
	require.NoError(t, err)

	err = mgr.Revoke(key1.ID)
	require.NoError(t, err)

	keys := mgr.List("user-1")
	assert.Len(t, keys, 1, "revoked key should be excluded from list")
}

func TestPermissions(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("perm-key", "proj-1", "user-1", []string{"read", "write"})
	require.NoError(t, err)

	assert.True(t, mgr.HasPermission(key.ID, "read"))
	assert.True(t, mgr.HasPermission(key.ID, "write"))
	assert.False(t, mgr.HasPermission(key.ID, "delete"))
}

func TestPermissionsWildcard(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("wildcard-key", "proj-1", "user-1", []string{"*"})
	require.NoError(t, err)

	assert.True(t, mgr.HasPermission(key.ID, "read"))
	assert.True(t, mgr.HasPermission(key.ID, "write"))
	assert.True(t, mgr.HasPermission(key.ID, "anything"))
}

func TestPermissionsRevokedKey(t *testing.T) {
	mgr := newTestManager()

	key, err := mgr.Generate("perm-key", "proj-1", "user-1", []string{"read"})
	require.NoError(t, err)

	err = mgr.Revoke(key.ID)
	require.NoError(t, err)

	assert.False(t, mgr.HasPermission(key.ID, "read"), "revoked key should not have permissions")
}

func TestPermissionsNonexistentKey(t *testing.T) {
	mgr := newTestManager()

	assert.False(t, mgr.HasPermission("nonexistent", "read"))
}

func TestInvalidKey(t *testing.T) {
	mgr := newTestManager()

	// Random string without prefix.
	_, err := mgr.Validate("totally-random-string")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key format")

	// Correct prefix but not a real key.
	_, err = mgr.Validate("nxo_deadbeefdeadbeefdeadbeef")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Empty string.
	_, err = mgr.Validate("")
	assert.Error(t, err)
}

func TestCount(t *testing.T) {
	mgr := newTestManager()

	assert.Equal(t, 0, mgr.Count("user-1"))

	key1, err := mgr.Generate("k1", "p1", "user-1", nil)
	require.NoError(t, err)
	_, err = mgr.Generate("k2", "p1", "user-1", nil)
	require.NoError(t, err)

	assert.Equal(t, 2, mgr.Count("user-1"))

	err = mgr.Revoke(key1.ID)
	require.NoError(t, err)

	assert.Equal(t, 1, mgr.Count("user-1"), "revoked key should not be counted")
}
