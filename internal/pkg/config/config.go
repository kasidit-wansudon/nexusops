package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// AppConfig holds all configuration for the NexusOps platform.
type AppConfig struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	Auth       AuthConfig       `yaml:"auth"`
	Docker     DockerConfig     `yaml:"docker"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	LogLevel   string           `yaml:"log_level"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
	MaxConns int    `yaml:"max_conns"`
	MinConns int    `yaml:"min_conns"`
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// Addr returns the Redis address string.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// AuthConfig holds authentication and OAuth settings.
type AuthConfig struct {
	GitHubClientID string        `yaml:"github_client_id"`
	GitHubSecret   string        `yaml:"github_secret"`
	GitLabClientID string        `yaml:"gitlab_client_id"`
	GitLabSecret   string        `yaml:"gitlab_secret"`
	JWTSecret      string        `yaml:"jwt_secret"`
	SessionTTL     time.Duration `yaml:"session_ttl"`
}

// DockerConfig holds Docker daemon settings.
type DockerConfig struct {
	Host       string `yaml:"host"`
	APIVersion string `yaml:"api_version"`
	TLSVerify  bool   `yaml:"tls_verify"`
	CertPath   string `yaml:"cert_path"`
	Registry   string `yaml:"registry"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

// KubernetesConfig holds Kubernetes connection settings.
type KubernetesConfig struct {
	Kubeconfig    string `yaml:"kubeconfig"`
	InCluster     bool   `yaml:"in_cluster"`
	Namespace     string `yaml:"namespace"`
	Context       string `yaml:"context"`
	ServiceAcct   string `yaml:"service_account"`
	LabelSelector string `yaml:"label_selector"`
}

// Load creates an AppConfig populated from environment variables, applying
// sensible defaults for any value not explicitly set.
func Load() *AppConfig {
	cfg := &AppConfig{
		Server: ServerConfig{
			Host:            getEnv("NEXUS_SERVER_HOST", "0.0.0.0"),
			Port:            getEnvInt("NEXUS_SERVER_PORT", 8080),
			ReadTimeout:     getEnvDuration("NEXUS_SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("NEXUS_SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvDuration("NEXUS_SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			Host:     getEnv("NEXUS_DB_HOST", "localhost"),
			Port:     getEnvInt("NEXUS_DB_PORT", 5432),
			Name:     getEnv("NEXUS_DB_NAME", "nexusops"),
			User:     getEnv("NEXUS_DB_USER", "nexusops"),
			Password: getEnv("NEXUS_DB_PASSWORD", ""),
			SSLMode:  getEnv("NEXUS_DB_SSLMODE", "disable"),
			MaxConns: getEnvInt("NEXUS_DB_MAX_CONNS", 25),
			MinConns: getEnvInt("NEXUS_DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Host:     getEnv("NEXUS_REDIS_HOST", "localhost"),
			Port:     getEnvInt("NEXUS_REDIS_PORT", 6379),
			Password: getEnv("NEXUS_REDIS_PASSWORD", ""),
			DB:       getEnvInt("NEXUS_REDIS_DB", 0),
			PoolSize: getEnvInt("NEXUS_REDIS_POOL_SIZE", 10),
		},
		Auth: AuthConfig{
			GitHubClientID: getEnv("NEXUS_GITHUB_CLIENT_ID", ""),
			GitHubSecret:   getEnv("NEXUS_GITHUB_SECRET", ""),
			GitLabClientID: getEnv("NEXUS_GITLAB_CLIENT_ID", ""),
			GitLabSecret:   getEnv("NEXUS_GITLAB_SECRET", ""),
			JWTSecret:      getEnv("NEXUS_JWT_SECRET", "change-me-in-production"),
			SessionTTL:     getEnvDuration("NEXUS_SESSION_TTL", 24*time.Hour),
		},
		Docker: DockerConfig{
			Host:       getEnv("NEXUS_DOCKER_HOST", "unix:///var/run/docker.sock"),
			APIVersion: getEnv("NEXUS_DOCKER_API_VERSION", "1.43"),
			TLSVerify:  getEnvBool("NEXUS_DOCKER_TLS_VERIFY", false),
			CertPath:   getEnv("NEXUS_DOCKER_CERT_PATH", ""),
			Registry:   getEnv("NEXUS_DOCKER_REGISTRY", ""),
			Username:   getEnv("NEXUS_DOCKER_USERNAME", ""),
			Password:   getEnv("NEXUS_DOCKER_PASSWORD", ""),
		},
		Kubernetes: KubernetesConfig{
			Kubeconfig:    getEnv("NEXUS_KUBECONFIG", ""),
			InCluster:     getEnvBool("NEXUS_K8S_IN_CLUSTER", false),
			Namespace:     getEnv("NEXUS_K8S_NAMESPACE", "default"),
			Context:       getEnv("NEXUS_K8S_CONTEXT", ""),
			ServiceAcct:   getEnv("NEXUS_K8S_SERVICE_ACCOUNT", ""),
			LabelSelector: getEnv("NEXUS_K8S_LABEL_SELECTOR", "app.kubernetes.io/managed-by=nexusops"),
		},
		LogLevel: getEnv("NEXUS_LOG_LEVEL", "info"),
	}
	return cfg
}

// LoadFromFile reads a YAML configuration file and merges it with
// environment-derived defaults. Environment variables take precedence.
func LoadFromFile(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	cfg := Load() // start with env-based defaults

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	// Environment variables override file-based values when set explicitly.
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides re-applies environment variables so they win over the YAML file.
func applyEnvOverrides(cfg *AppConfig) {
	if v := os.Getenv("NEXUS_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("NEXUS_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("NEXUS_DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("NEXUS_DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = p
		}
	}
	if v := os.Getenv("NEXUS_DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("NEXUS_DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("NEXUS_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("NEXUS_REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("NEXUS_REDIS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = p
		}
	}
	if v := os.Getenv("NEXUS_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("NEXUS_GITHUB_CLIENT_ID"); v != "" {
		cfg.Auth.GitHubClientID = v
	}
	if v := os.Getenv("NEXUS_GITHUB_SECRET"); v != "" {
		cfg.Auth.GitHubSecret = v
	}
	if v := os.Getenv("NEXUS_GITLAB_CLIENT_ID"); v != "" {
		cfg.Auth.GitLabClientID = v
	}
	if v := os.Getenv("NEXUS_GITLAB_SECRET"); v != "" {
		cfg.Auth.GitLabSecret = v
	}
}

// Validate checks that essential configuration values are present.
func (c *AppConfig) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}
	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("a secure JWT secret must be configured for production")
	}
	return nil
}

// ServerAddr returns the formatted listen address.
func (c *AppConfig) ServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// --- helper functions ---

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
