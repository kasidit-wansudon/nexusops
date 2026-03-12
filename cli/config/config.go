package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type CLIConfig struct {
	APIURL      string            `json:"api_url"`
	APIKey      string            `json:"api_key"`
	DefaultTeam string            `json:"default_team"`
	Profiles    map[string]Profile `json:"profiles"`
	Active      string            `json:"active_profile"`
}

type Profile struct {
	Name   string `json:"name"`
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nexusops/config.json"
	}
	return filepath.Join(home, ".nexusops", "config.json")
}

func Load() (*CLIConfig, error) {
	path := DefaultConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CLIConfig{
				APIURL:   "http://localhost:8080",
				Profiles: make(map[string]Profile),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg CLIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}

	return &cfg, nil
}

func (c *CLIConfig) Save() error {
	path := DefaultConfigPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (c *CLIConfig) GetActiveProfile() *Profile {
	if c.Active == "" {
		return nil
	}
	if p, ok := c.Profiles[c.Active]; ok {
		return &p
	}
	return nil
}

func (c *CLIConfig) SetProfile(name string, profile Profile) {
	c.Profiles[name] = profile
}

func (c *CLIConfig) DeleteProfile(name string) {
	delete(c.Profiles, name)
	if c.Active == name {
		c.Active = ""
	}
}
