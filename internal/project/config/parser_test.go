package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fullConfigYAML = `
name: nexusops-app
version: "1.0.0"
build:
  dockerfile: Dockerfile.prod
  context: ./app
  cache:
    - /tmp/go-cache
    - /tmp/npm-cache
  steps:
    - name: compile
      command: go build -o bin/app ./cmd/server
    - name: test
      command: go test ./...
deploy:
  provider: kubernetes
  strategy: blue-green
  replicas: 3
  port: 9090
  health_check:
    path: /healthz
    interval: 10s
    timeout: 3s
    retries: 5
  resources:
    cpu: "500m"
    memory: "256Mi"
  env:
    NODE_ENV: production
    LOG_LEVEL: info
pipeline:
  stages:
    - name: build
      steps:
        - name: compile
          image: golang:1.22
          commands:
            - go build ./...
          timeout: 10m
    - name: deploy
      steps:
        - name: rollout
          image: kubectl:latest
          commands:
            - kubectl apply -f deploy.yaml
          depends_on:
            - compile
notifications:
  slack:
    webhook_url: https://hooks.slack.com/services/xxx
    channel: "#deploys"
  discord:
    webhook_url: https://discord.com/api/webhooks/xxx
  email:
    - ops@example.com
`

func TestParseValidConfig(t *testing.T) {
	cfg, err := Parse([]byte(fullConfigYAML))
	require.NoError(t, err)

	assert.Equal(t, "nexusops-app", cfg.Name)
	assert.Equal(t, "1.0.0", cfg.Version)

	// Build
	assert.Equal(t, "Dockerfile.prod", cfg.Build.Dockerfile)
	assert.Equal(t, "./app", cfg.Build.Context)
	assert.Len(t, cfg.Build.Cache, 2)
	assert.Len(t, cfg.Build.Steps, 2)
	assert.Equal(t, "compile", cfg.Build.Steps[0].Name)

	// Deploy
	assert.Equal(t, "kubernetes", cfg.Deploy.Provider)
	assert.Equal(t, "blue-green", cfg.Deploy.Strategy)
	assert.Equal(t, 3, cfg.Deploy.Replicas)
	assert.Equal(t, 9090, cfg.Deploy.Port)
	assert.Equal(t, "/healthz", cfg.Deploy.HealthCheck.Path)
	assert.Equal(t, "10s", cfg.Deploy.HealthCheck.Interval)
	assert.Equal(t, "3s", cfg.Deploy.HealthCheck.Timeout)
	assert.Equal(t, 5, cfg.Deploy.HealthCheck.Retries)
	assert.Equal(t, "500m", cfg.Deploy.Resources.CPU)
	assert.Equal(t, "256Mi", cfg.Deploy.Resources.Memory)
	assert.Equal(t, "production", cfg.Deploy.Env["NODE_ENV"])

	// Pipeline
	assert.Len(t, cfg.Pipeline.Stages, 2)
	assert.Equal(t, "build", cfg.Pipeline.Stages[0].Name)
	assert.Equal(t, "compile", cfg.Pipeline.Stages[0].Steps[0].Name)
	assert.Equal(t, "golang:1.22", cfg.Pipeline.Stages[0].Steps[0].Image)
	assert.Equal(t, "10m", cfg.Pipeline.Stages[0].Steps[0].Timeout)
	assert.Equal(t, []string{"compile"}, cfg.Pipeline.Stages[1].Steps[0].DependsOn)

	// Notifications
	assert.Equal(t, "#deploys", cfg.Notifications.Slack.Channel)
	assert.NotEmpty(t, cfg.Notifications.Discord.WebhookURL)
	assert.Contains(t, cfg.Notifications.Email, "ops@example.com")
}

func TestParseMinimalConfig(t *testing.T) {
	minimal := `name: minimal-project`
	cfg, err := Parse([]byte(minimal))
	require.NoError(t, err)

	assert.Equal(t, "minimal-project", cfg.Name)

	// Defaults should be applied.
	assert.Equal(t, "docker", cfg.Deploy.Provider)
	assert.Equal(t, "rolling", cfg.Deploy.Strategy)
	assert.Equal(t, 1, cfg.Deploy.Replicas)
	assert.Equal(t, 8080, cfg.Deploy.Port)
	assert.Equal(t, "/health", cfg.Deploy.HealthCheck.Path)
	assert.Equal(t, "30s", cfg.Deploy.HealthCheck.Interval)
	assert.Equal(t, "5s", cfg.Deploy.HealthCheck.Timeout)
	assert.Equal(t, 3, cfg.Deploy.HealthCheck.Retries)
	assert.Equal(t, "Dockerfile", cfg.Build.Dockerfile)
	assert.Equal(t, ".", cfg.Build.Context)
	assert.NotNil(t, cfg.Deploy.Env, "Env map should be initialized even when empty")
}

func TestValidateConfig(t *testing.T) {
	cfg, err := Parse([]byte(fullConfigYAML))
	require.NoError(t, err)

	err = Validate(cfg)
	assert.NoError(t, err, "a fully valid config must pass validation")
}

func TestValidateConfigMissingName(t *testing.T) {
	yamlData := `
version: "1.0.0"
`
	cfg, err := Parse([]byte(yamlData))
	require.NoError(t, err)

	err = Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project name is required")
}

func TestValidateConfigInvalidRuntime(t *testing.T) {
	// Test invalid provider.
	yamlData := `
name: bad-provider
version: "1.0.0"
deploy:
  provider: heroku
  strategy: rolling
  replicas: 1
  port: 8080
  health_check:
    path: /health
    interval: 30s
    timeout: 5s
    retries: 3
`
	cfg, err := Parse([]byte(yamlData))
	require.NoError(t, err)

	err = Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid deployment provider")

	// Test invalid strategy.
	yamlData2 := `
name: bad-strategy
version: "1.0.0"
deploy:
  provider: docker
  strategy: recreate
  replicas: 1
  port: 8080
  health_check:
    path: /health
    interval: 30s
    timeout: 5s
    retries: 3
`
	cfg2, err := Parse([]byte(yamlData2))
	require.NoError(t, err)

	err = Validate(cfg2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid deployment strategy")
}

func TestParseConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nexusops.yaml")

	content := `
name: file-parsed-project
version: "2.0.0"
deploy:
  provider: docker
  strategy: canary
  replicas: 2
  port: 3000
  health_check:
    path: /ready
    interval: 15s
    timeout: 5s
    retries: 4
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := ParseFile(filePath)
	require.NoError(t, err)

	assert.Equal(t, "file-parsed-project", cfg.Name)
	assert.Equal(t, "2.0.0", cfg.Version)
	assert.Equal(t, "canary", cfg.Deploy.Strategy)
	assert.Equal(t, 2, cfg.Deploy.Replicas)
	assert.Equal(t, 3000, cfg.Deploy.Port)
	assert.Equal(t, "/ready", cfg.Deploy.HealthCheck.Path)

	// Parsing a nonexistent file should fail.
	_, err = ParseFile(filepath.Join(tmpDir, "does-not-exist.yaml"))
	assert.Error(t, err)
}

func TestParseConfigInvalidYAML(t *testing.T) {
	invalid := `
name: broken
  bad_indent: [this is not valid
    yaml: {{{{
`
	_, err := Parse([]byte(invalid))
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "failed to parse YAML"),
		"error should mention YAML parse failure, got: %s", err.Error())

	// Completely empty input.
	_, err = Parse([]byte{})
	assert.ErrorIs(t, err, ErrEmptyConfig)

	// Nil input.
	_, err = Parse(nil)
	assert.ErrorIs(t, err, ErrEmptyConfig)
}
