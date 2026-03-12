package apikey

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManagerPanicsOnEmptyKey(t *testing.T) {
	assert.Panics(t, func() {
		NewManager([]byte{})
	})
}

func TestGenerateAndValidate(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	key, err := m.Generate("my-key", "proj-1", "user-1", []string{"read", "write"})
	require.NoError(t, err)
	assert.NotEmpty(t, key.ID)
	assert.True(t, strings.HasPrefix(key.Key, "nxo_"))
	assert.Equal(t, "my-key", key.Name)
	assert.Equal(t, "proj-1", key.ProjectID)
	assert.Equal(t, "user-1", key.UserID)
	assert.Equal(t, []string{"read", "write"}, key.Permissions)
	assert.False(t, key.Revoked)

	// Validate the key
	validated, err := m.Validate(key.Key)
	require.NoError(t, err)
	assert.Equal(t, key.ID, validated.ID)
	assert.Empty(t, validated.Key) // plaintext should not be returned
	assert.False(t, validated.LastUsedAt.IsZero())
}

func TestGenerateValidation(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	_, err := m.Generate("", "proj", "user", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")

	_, err = m.Generate("name", "proj", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID must not be empty")
}

func TestValidateInvalidFormat(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	_, err := m.Validate("invalid-key-without-prefix")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key format")
}

func TestValidateNonexistentKey(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	_, err := m.Validate("nxo_nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestRevokeAndValidate(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	key, err := m.Generate("my-key", "proj-1", "user-1", nil)
	require.NoError(t, err)

	err = m.Revoke(key.ID)
	require.NoError(t, err)

	_, err = m.Validate(key.Key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRevokeNonexistent(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	err := m.Revoke("nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListAndCount(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	_, _ = m.Generate("key-1", "proj", "user-1", nil)
	_, _ = m.Generate("key-2", "proj", "user-1", nil)
	_, _ = m.Generate("key-3", "proj", "user-2", nil)

	assert.Equal(t, 2, m.Count("user-1"))
	assert.Equal(t, 1, m.Count("user-2"))
	assert.Equal(t, 0, m.Count("user-3"))

	list := m.List("user-1")
	assert.Len(t, list, 2)
	for _, k := range list {
		assert.Empty(t, k.Key) // plaintext should be stripped
		assert.Equal(t, "user-1", k.UserID)
	}
}

func TestHasPermission(t *testing.T) {
	m := NewManager([]byte("test-encryption-key-32-bytes!!!!"))

	key, _ := m.Generate("key-1", "proj", "user-1", []string{"read", "write"})
	wildcard, _ := m.Generate("key-2", "proj", "user-1", []string{"*"})

	assert.True(t, m.HasPermission(key.ID, "read"))
	assert.True(t, m.HasPermission(key.ID, "write"))
	assert.False(t, m.HasPermission(key.ID, "admin"))

	// Wildcard grants all permissions
	assert.True(t, m.HasPermission(wildcard.ID, "anything"))
	assert.True(t, m.HasPermission(wildcard.ID, "admin"))

	// Nonexistent key
	assert.False(t, m.HasPermission("nonexistent", "read"))

	// Revoked key
	_ = m.Revoke(key.ID)
	assert.False(t, m.HasPermission(key.ID, "read"))
}
