package env

import (
	"testing"

	"github.com/kasidit-wansudon/nexusops/internal/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKey generates a valid 32-byte encryption key for tests.
func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	return key
}

func TestNewManager(t *testing.T) {
	key := testKey(t)
	mgr, err := NewManager(key)
	require.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.store, "store map must be initialised")
}

func TestNewManagerInvalidKey(t *testing.T) {
	// Too short.
	_, err := NewManager([]byte("short-key"))
	assert.ErrorIs(t, err, ErrInvalidKeyLength)

	// Too long.
	longKey := make([]byte, 64)
	_, err = NewManager(longKey)
	assert.ErrorIs(t, err, ErrInvalidKeyLength)

	// Empty.
	_, err = NewManager(nil)
	assert.ErrorIs(t, err, ErrInvalidKeyLength)
}

func TestSetAndGet(t *testing.T) {
	key := testKey(t)
	mgr, err := NewManager(key)
	require.NoError(t, err)

	projectID := "proj-1"
	envName := "production"
	varKey := "DATABASE_URL"
	varValue := "postgres://user:pass@host:5432/db"

	err = mgr.Set(projectID, varKey, varValue, envName)
	require.NoError(t, err)

	got, err := mgr.Get(projectID, varKey, envName)
	require.NoError(t, err)
	assert.Equal(t, varValue, got, "Get must return the original plaintext value")
}

func TestSetAndList(t *testing.T) {
	key := testKey(t)
	mgr, err := NewManager(key)
	require.NoError(t, err)

	projectID := "proj-list"
	envName := "staging"

	vars := map[string]string{
		"API_KEY":    "sk-abcdefghijklmnop",
		"SECRET_KEY": "super-secret-value-1234",
		"DB_PASS":    "mydbpassword",
	}

	for k, v := range vars {
		err := mgr.Set(projectID, k, v, envName)
		require.NoError(t, err)
	}

	listed, err := mgr.List(projectID, envName)
	require.NoError(t, err)
	assert.Len(t, listed, len(vars), "List must return all set variables")

	// Values should be masked - not equal to the original or encrypted values.
	for _, ev := range listed {
		originalValue := vars[ev.Key]
		assert.NotEqual(t, originalValue, ev.Value, "listed value for %s should be masked", ev.Key)

		// Masked values for strings > 4 chars should start with first 4 chars of plaintext.
		if len(originalValue) > 4 {
			assert.Equal(t, originalValue[:4], ev.Value[:4],
				"masked value should start with first 4 chars of plaintext for key %s", ev.Key)
			assert.Contains(t, ev.Value, "****", "masked value should contain asterisks")
		}
	}
}

func TestDeleteVariable(t *testing.T) {
	key := testKey(t)
	mgr, err := NewManager(key)
	require.NoError(t, err)

	projectID := "proj-del"
	envName := "dev"

	err = mgr.Set(projectID, "TO_DELETE", "value123", envName)
	require.NoError(t, err)

	// Verify it exists.
	got, err := mgr.Get(projectID, "TO_DELETE", envName)
	require.NoError(t, err)
	assert.Equal(t, "value123", got)

	// Delete it.
	err = mgr.Delete(projectID, "TO_DELETE", envName)
	require.NoError(t, err)

	// Verify it is gone.
	_, err = mgr.Get(projectID, "TO_DELETE", envName)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestDeleteNonExistent(t *testing.T) {
	key := testKey(t)
	mgr, err := NewManager(key)
	require.NoError(t, err)

	// Delete from a project that does not exist.
	err = mgr.Delete("nonexistent-proj", "SOME_VAR", "production")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrProjectNotFound)

	// Add a variable, then try to delete a different key.
	err = mgr.Set("proj-x", "EXISTS", "val", "dev")
	require.NoError(t, err)

	err = mgr.Delete("proj-x", "DOES_NOT_EXIST", "dev")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrVarNotFound)
}

func TestBulkImportExport(t *testing.T) {
	key := testKey(t)
	mgr, err := NewManager(key)
	require.NoError(t, err)

	projectID := "proj-bulk"
	envName := "production"

	input := map[string]string{
		"DB_HOST":     "db.example.com",
		"DB_PORT":     "5432",
		"DB_USER":     "admin",
		"DB_PASSWORD": "s3cret!@#",
		"REDIS_URL":   "redis://localhost:6379",
	}

	err = mgr.Import(projectID, envName, input)
	require.NoError(t, err)

	// Count should match.
	assert.Equal(t, len(input), mgr.Count(projectID, envName))

	// Export and verify all values match.
	exported, err := mgr.Export(projectID, envName)
	require.NoError(t, err)
	assert.Equal(t, input, exported, "exported map must equal the imported map")
}

func TestMultipleEnvironments(t *testing.T) {
	key := testKey(t)
	mgr, err := NewManager(key)
	require.NoError(t, err)

	projectID := "proj-multi"
	varKey := "DATABASE_URL"

	devValue := "postgres://localhost:5432/dev"
	prodValue := "postgres://prod-host:5432/prod"

	err = mgr.Set(projectID, varKey, devValue, "development")
	require.NoError(t, err)
	err = mgr.Set(projectID, varKey, prodValue, "production")
	require.NoError(t, err)

	// Retrieve from each environment independently.
	gotDev, err := mgr.Get(projectID, varKey, "development")
	require.NoError(t, err)
	assert.Equal(t, devValue, gotDev)

	gotProd, err := mgr.Get(projectID, varKey, "production")
	require.NoError(t, err)
	assert.Equal(t, prodValue, gotProd)

	// The two environments should be listed.
	envs, err := mgr.Environments(projectID)
	require.NoError(t, err)
	assert.Len(t, envs, 2)
	assert.ElementsMatch(t, []string{"development", "production"}, envs)

	// Counts per environment.
	assert.Equal(t, 1, mgr.Count(projectID, "development"))
	assert.Equal(t, 1, mgr.Count(projectID, "production"))
}
