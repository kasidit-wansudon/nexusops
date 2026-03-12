package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	assert.Contains(t, path, ".nexusops")
	assert.Contains(t, path, "config.json")
}

func TestLoadReturnsDefaultsWhenMissing(t *testing.T) {
	// Override HOME so DefaultConfigPath points to a nonexistent location
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", cfg.APIURL)
	assert.NotNil(t, cfg.Profiles)
	assert.Empty(t, cfg.Profiles)
	assert.Empty(t, cfg.Active)
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &CLIConfig{
		APIURL:      "https://api.example.com",
		APIKey:      "key-123",
		DefaultTeam: "team-alpha",
		Profiles:    make(map[string]Profile),
		Active:      "prod",
	}
	cfg.Profiles["prod"] = Profile{
		Name:   "prod",
		APIURL: "https://prod.example.com",
		APIKey: "prod-key",
	}

	err := cfg.Save()
	require.NoError(t, err)

	// Verify the file was created with correct permissions
	path := DefaultConfigPath()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Verify valid JSON
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	// Load back
	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com", loaded.APIURL)
	assert.Equal(t, "key-123", loaded.APIKey)
	assert.Equal(t, "team-alpha", loaded.DefaultTeam)
	assert.Equal(t, "prod", loaded.Active)
	require.Contains(t, loaded.Profiles, "prod")
	assert.Equal(t, "prod-key", loaded.Profiles["prod"].APIKey)
}

func TestGetActiveProfile(t *testing.T) {
	cfg := &CLIConfig{
		Profiles: map[string]Profile{
			"staging": {Name: "staging", APIURL: "https://staging.io", APIKey: "stg-key"},
		},
		Active: "staging",
	}

	p := cfg.GetActiveProfile()
	require.NotNil(t, p)
	assert.Equal(t, "staging", p.Name)
	assert.Equal(t, "stg-key", p.APIKey)

	// No active profile
	cfg.Active = ""
	assert.Nil(t, cfg.GetActiveProfile())

	// Active profile name doesn't match any profile
	cfg.Active = "nonexistent"
	assert.Nil(t, cfg.GetActiveProfile())
}

func TestSetAndDeleteProfile(t *testing.T) {
	cfg := &CLIConfig{
		Profiles: make(map[string]Profile),
	}

	cfg.SetProfile("dev", Profile{Name: "dev", APIURL: "http://dev.local", APIKey: "dev-key"})
	assert.Contains(t, cfg.Profiles, "dev")
	assert.Equal(t, "dev-key", cfg.Profiles["dev"].APIKey)

	// Overwrite
	cfg.SetProfile("dev", Profile{Name: "dev", APIURL: "http://dev2.local", APIKey: "dev-key-2"})
	assert.Equal(t, "dev-key-2", cfg.Profiles["dev"].APIKey)

	// Delete
	cfg.Active = "dev"
	cfg.DeleteProfile("dev")
	assert.NotContains(t, cfg.Profiles, "dev")
	assert.Empty(t, cfg.Active, "deleting the active profile should clear Active")

	// Delete nonexistent is a no-op
	cfg.DeleteProfile("nonexistent")
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Directory should not exist yet
	configDir := filepath.Join(tmp, ".nexusops")
	_, err := os.Stat(configDir)
	assert.True(t, os.IsNotExist(err))

	cfg := &CLIConfig{
		APIURL:   "http://localhost:9090",
		Profiles: make(map[string]Profile),
	}
	require.NoError(t, cfg.Save())

	// Now it should exist
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
