package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey() []byte {
	return []byte("01234567890123456789012345678901") // 32 bytes
}

func TestNewManagerInvalidKeyLength(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{"too short", []byte("short")},
		{"too long", make([]byte, 64)},
		{"empty", []byte{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewManager(tc.key)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidKeyLength)
		})
	}
}

func TestSetAndGet(t *testing.T) {
	m, err := NewManager(validKey())
	require.NoError(t, err)

	err = m.Set("proj-1", "DB_HOST", "localhost:5432", "production")
	require.NoError(t, err)

	val, err := m.Get("proj-1", "DB_HOST", "production")
	require.NoError(t, err)
	assert.Equal(t, "localhost:5432", val)
}

func TestSetValidation(t *testing.T) {
	m, err := NewManager(validKey())
	require.NoError(t, err)

	tests := []struct {
		name        string
		projectID   string
		key         string
		value       string
		environment string
		expectedErr error
	}{
		{"empty project", "", "KEY", "val", "dev", ErrEmptyProjectID},
		{"empty key", "proj", "", "val", "dev", ErrEmptyKey},
		{"empty env", "proj", "KEY", "val", "", ErrEmptyEnvironment},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := m.Set(tc.projectID, tc.key, tc.value, tc.environment)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.expectedErr)
		})
	}
}

func TestGetNotFound(t *testing.T) {
	m, err := NewManager(validKey())
	require.NoError(t, err)

	// Project not found
	_, err = m.Get("nonexistent", "KEY", "dev")
	assert.ErrorIs(t, err, ErrProjectNotFound)

	// Variable not found
	err = m.Set("proj-1", "EXISTING", "val", "dev")
	require.NoError(t, err)

	_, err = m.Get("proj-1", "MISSING", "dev")
	assert.ErrorIs(t, err, ErrVarNotFound)
}

func TestDeleteAndCount(t *testing.T) {
	m, err := NewManager(validKey())
	require.NoError(t, err)

	_ = m.Set("proj-1", "A", "1", "dev")
	_ = m.Set("proj-1", "B", "2", "dev")
	_ = m.Set("proj-1", "C", "3", "staging")

	assert.Equal(t, 2, m.Count("proj-1", "dev"))
	assert.Equal(t, 1, m.Count("proj-1", "staging"))

	err = m.Delete("proj-1", "A", "dev")
	require.NoError(t, err)
	assert.Equal(t, 1, m.Count("proj-1", "dev"))

	// Delete non-existent
	err = m.Delete("proj-1", "NOPE", "dev")
	assert.ErrorIs(t, err, ErrVarNotFound)

	// Delete from non-existent project
	err = m.Delete("nonexistent", "A", "dev")
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestListMasksValues(t *testing.T) {
	m, err := NewManager(validKey())
	require.NoError(t, err)

	_ = m.Set("proj-1", "SECRET", "super-secret-value", "production")
	_ = m.Set("proj-1", "SHORT", "ab", "production")

	vars, err := m.List("proj-1", "production")
	require.NoError(t, err)
	require.Len(t, vars, 2)

	for _, v := range vars {
		if v.Key == "SECRET" {
			// Should show first 4 chars then asterisks
			assert.Equal(t, "supe**************", v.Value)
		}
		if v.Key == "SHORT" {
			// <= 4 chars should be fully masked
			assert.Equal(t, "**", v.Value)
		}
	}
}

func TestExportAndImport(t *testing.T) {
	m, err := NewManager(validKey())
	require.NoError(t, err)

	input := map[string]string{
		"DB_HOST":     "localhost",
		"DB_PASSWORD": "s3cret",
		"API_KEY":     "abc123",
	}

	err = m.Import("proj-1", "dev", input)
	require.NoError(t, err)
	assert.Equal(t, 3, m.Count("proj-1", "dev"))

	exported, err := m.Export("proj-1", "dev")
	require.NoError(t, err)
	assert.Equal(t, input, exported)

	// Export for non-existent project returns empty map
	exported, err = m.Export("nonexistent", "dev")
	require.NoError(t, err)
	assert.Empty(t, exported)
}

func TestEnvironments(t *testing.T) {
	m, err := NewManager(validKey())
	require.NoError(t, err)

	_ = m.Set("proj-1", "A", "1", "dev")
	_ = m.Set("proj-1", "B", "2", "staging")
	_ = m.Set("proj-1", "C", "3", "production")
	_ = m.Set("proj-1", "D", "4", "dev") // duplicate env

	envs, err := m.Environments("proj-1")
	require.NoError(t, err)
	assert.Len(t, envs, 3)
	assert.ElementsMatch(t, []string{"dev", "staging", "production"}, envs)

	// Non-existent project returns empty
	envs, err = m.Environments("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, envs)

	// Empty project ID returns error
	_, err = m.Environments("")
	assert.ErrorIs(t, err, ErrEmptyProjectID)
}
