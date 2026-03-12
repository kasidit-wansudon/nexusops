package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clearEnv unsets all NEXUS_* environment variables so tests get clean defaults.
func clearEnv(t *testing.T) {
	t.Helper()
	envVars := []string{
		"NEXUS_SERVER_HOST", "NEXUS_SERVER_PORT",
		"NEXUS_SERVER_READ_TIMEOUT", "NEXUS_SERVER_WRITE_TIMEOUT", "NEXUS_SERVER_SHUTDOWN_TIMEOUT",
		"NEXUS_DB_HOST", "NEXUS_DB_PORT", "NEXUS_DB_NAME", "NEXUS_DB_USER", "NEXUS_DB_PASSWORD", "NEXUS_DB_SSLMODE",
		"NEXUS_DB_MAX_CONNS", "NEXUS_DB_MIN_CONNS",
		"NEXUS_REDIS_HOST", "NEXUS_REDIS_PORT", "NEXUS_REDIS_PASSWORD", "NEXUS_REDIS_DB", "NEXUS_REDIS_POOL_SIZE",
		"NEXUS_JWT_SECRET", "NEXUS_SESSION_TTL",
		"NEXUS_DOCKER_HOST", "NEXUS_DOCKER_API_VERSION", "NEXUS_DOCKER_TLS_VERIFY",
		"NEXUS_DOCKER_CERT_PATH", "NEXUS_DOCKER_REGISTRY", "NEXUS_DOCKER_USERNAME", "NEXUS_DOCKER_PASSWORD",
		"NEXUS_K8S_IN_CLUSTER", "NEXUS_K8S_NAMESPACE", "NEXUS_K8S_CONTEXT", "NEXUS_K8S_LABEL_SELECTOR",
		"NEXUS_K8S_SERVICE_ACCOUNT", "NEXUS_KUBECONFIG",
		"NEXUS_LOG_LEVEL",
		"NEXUS_GITHUB_CLIENT_ID", "NEXUS_GITHUB_SECRET",
		"NEXUS_GITLAB_CLIENT_ID", "NEXUS_GITLAB_SECRET",
	}
	for _, e := range envVars {
		os.Unsetenv(e)
	}
}

func TestLoad(t *testing.T) {
	clearEnv(t)

	cfg := Load()
	if cfg == nil {
		t.Fatal("Load returned nil")
	}
	if cfg.Server.Host == "" {
		t.Error("Server.Host is empty")
	}
	if cfg.Server.Port <= 0 {
		t.Error("Server.Port is not positive")
	}
	if cfg.Database.Host == "" {
		t.Error("Database.Host is empty")
	}
	if cfg.Database.Name == "" {
		t.Error("Database.Name is empty")
	}
	if cfg.LogLevel == "" {
		t.Error("LogLevel is empty")
	}
}

func TestConfigDefaults(t *testing.T) {
	clearEnv(t)

	cfg := Load()

	// Server defaults.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want %v", cfg.Server.ReadTimeout, 30*time.Second)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want %v", cfg.Server.WriteTimeout, 30*time.Second)
	}
	if cfg.Server.ShutdownTimeout != 15*time.Second {
		t.Errorf("Server.ShutdownTimeout = %v, want %v", cfg.Server.ShutdownTimeout, 15*time.Second)
	}

	// Database defaults.
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "localhost")
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want %d", cfg.Database.Port, 5432)
	}
	if cfg.Database.Name != "nexusops" {
		t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "nexusops")
	}
	if cfg.Database.User != "nexusops" {
		t.Errorf("Database.User = %q, want %q", cfg.Database.User, "nexusops")
	}
	if cfg.Database.SSLMode != "disable" {
		t.Errorf("Database.SSLMode = %q, want %q", cfg.Database.SSLMode, "disable")
	}
	if cfg.Database.MaxConns != 25 {
		t.Errorf("Database.MaxConns = %d, want %d", cfg.Database.MaxConns, 25)
	}
	if cfg.Database.MinConns != 5 {
		t.Errorf("Database.MinConns = %d, want %d", cfg.Database.MinConns, 5)
	}

	// Redis defaults.
	if cfg.Redis.Host != "localhost" {
		t.Errorf("Redis.Host = %q, want %q", cfg.Redis.Host, "localhost")
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("Redis.Port = %d, want %d", cfg.Redis.Port, 6379)
	}
	if cfg.Redis.PoolSize != 10 {
		t.Errorf("Redis.PoolSize = %d, want %d", cfg.Redis.PoolSize, 10)
	}

	// Auth defaults.
	if cfg.Auth.JWTSecret != "change-me-in-production" {
		t.Errorf("Auth.JWTSecret = %q, want %q", cfg.Auth.JWTSecret, "change-me-in-production")
	}
	if cfg.Auth.SessionTTL != 24*time.Hour {
		t.Errorf("Auth.SessionTTL = %v, want %v", cfg.Auth.SessionTTL, 24*time.Hour)
	}

	// Docker defaults.
	if cfg.Docker.Host != "unix:///var/run/docker.sock" {
		t.Errorf("Docker.Host = %q, want %q", cfg.Docker.Host, "unix:///var/run/docker.sock")
	}
	if cfg.Docker.APIVersion != "1.43" {
		t.Errorf("Docker.APIVersion = %q, want %q", cfg.Docker.APIVersion, "1.43")
	}

	// Kubernetes defaults.
	if cfg.Kubernetes.Namespace != "default" {
		t.Errorf("Kubernetes.Namespace = %q, want %q", cfg.Kubernetes.Namespace, "default")
	}

	// LogLevel.
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestConfigEnvironmentOverride(t *testing.T) {
	clearEnv(t)

	os.Setenv("NEXUS_SERVER_HOST", "127.0.0.1")
	os.Setenv("NEXUS_SERVER_PORT", "9090")
	os.Setenv("NEXUS_DB_HOST", "db.prod.internal")
	os.Setenv("NEXUS_DB_PORT", "5433")
	os.Setenv("NEXUS_DB_NAME", "prod_db")
	os.Setenv("NEXUS_LOG_LEVEL", "debug")
	defer clearEnv(t)

	cfg := Load()

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Database.Host != "db.prod.internal" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.prod.internal")
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("Database.Port = %d, want %d", cfg.Database.Port, 5433)
	}
	if cfg.Database.Name != "prod_db" {
		t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "prod_db")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestConfigHelpers(t *testing.T) {
	clearEnv(t)
	cfg := Load()

	// ServerAddr.
	addr := cfg.ServerAddr()
	if addr != "0.0.0.0:8080" {
		t.Errorf("ServerAddr = %q, want %q", addr, "0.0.0.0:8080")
	}

	// Database DSN.
	dsn := cfg.Database.DSN()
	if !strings.Contains(dsn, "host=localhost") {
		t.Errorf("DSN = %q, want it to contain 'host=localhost'", dsn)
	}
	if !strings.Contains(dsn, "port=5432") {
		t.Errorf("DSN = %q, want it to contain 'port=5432'", dsn)
	}
	if !strings.Contains(dsn, "dbname=nexusops") {
		t.Errorf("DSN = %q, want it to contain 'dbname=nexusops'", dsn)
	}

	// Redis Addr.
	redisAddr := cfg.Redis.Addr()
	if redisAddr != "localhost:6379" {
		t.Errorf("Redis.Addr = %q, want %q", redisAddr, "localhost:6379")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(c *AppConfig)
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "default config fails due to JWT",
			mutate:    func(c *AppConfig) {},
			wantErr:   true,
			errSubstr: "JWT secret",
		},
		{
			name: "valid config passes",
			mutate: func(c *AppConfig) {
				c.Auth.JWTSecret = "a-very-secure-secret-key-for-testing"
			},
			wantErr: false,
		},
		{
			name: "invalid port zero",
			mutate: func(c *AppConfig) {
				c.Auth.JWTSecret = "secure"
				c.Server.Port = 0
			},
			wantErr:   true,
			errSubstr: "invalid server port",
		},
		{
			name: "empty database host",
			mutate: func(c *AppConfig) {
				c.Auth.JWTSecret = "secure"
				c.Database.Host = ""
			},
			wantErr:   true,
			errSubstr: "database host",
		},
		{
			name: "empty database name",
			mutate: func(c *AppConfig) {
				c.Auth.JWTSecret = "secure"
				c.Database.Name = ""
			},
			wantErr:   true,
			errSubstr: "database name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			cfg := Load()
			tc.mutate(cfg)

			err := cfg.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tc.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	clearEnv(t)

	yamlContent := `
server:
  host: "10.0.0.1"
  port: 3000
database:
  host: "db.local"
  port: 5433
  name: "testdb"
log_level: "warn"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Server.Host != "10.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "10.0.0.1")
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 3000)
	}
	if cfg.Database.Host != "db.local" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.local")
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("Database.Port = %d, want %d", cfg.Database.Port, 5433)
	}
	if cfg.Database.Name != "testdb" {
		t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "testdb")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "warn")
	}
}
