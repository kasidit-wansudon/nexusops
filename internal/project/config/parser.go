package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrEmptyConfig       = errors.New("config: empty configuration data")
	ErrMissingName       = errors.New("config: project name is required")
	ErrMissingVersion    = errors.New("config: version is required")
	ErrInvalidStrategy   = errors.New("config: invalid deployment strategy")
	ErrInvalidProvider   = errors.New("config: invalid deployment provider")
	ErrInvalidReplicas   = errors.New("config: replicas must be greater than zero")
	ErrInvalidPort       = errors.New("config: port must be between 1 and 65535")
	ErrInvalidRetries    = errors.New("config: health check retries must be greater than zero")
	ErrMissingStageName  = errors.New("config: stage name is required")
	ErrMissingStepName   = errors.New("config: pipeline step name is required")
	ErrMissingStepImage  = errors.New("config: pipeline step image is required")
	ErrCircularDependency = errors.New("config: circular dependency detected in pipeline steps")
	ErrInvalidTimeout    = errors.New("config: invalid timeout duration")
)

var validStrategies = map[string]bool{
	"rolling":    true,
	"blue-green": true,
	"canary":     true,
}

var validProviders = map[string]bool{
	"docker":     true,
	"kubernetes": true,
}

// NexusOpsConfig is the top-level configuration for a nexusops.yaml file.
type NexusOpsConfig struct {
	Name          string         `yaml:"name"`
	Version       string         `yaml:"version"`
	Build         BuildConfig    `yaml:"build"`
	Deploy        DeployConfig   `yaml:"deploy"`
	Pipeline      PipelineConfig `yaml:"pipeline"`
	Notifications Notifications  `yaml:"notifications"`
}

// BuildConfig holds build-related settings.
type BuildConfig struct {
	Steps      []BuildStep `yaml:"steps"`
	Dockerfile string      `yaml:"dockerfile"`
	Context    string      `yaml:"context"`
	Cache      []string    `yaml:"cache"`
}

// BuildStep represents a single build step command.
type BuildStep struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

// DeployConfig holds deployment settings.
type DeployConfig struct {
	Provider    string            `yaml:"provider"`
	Strategy    string            `yaml:"strategy"`
	Replicas    int               `yaml:"replicas"`
	Port        int               `yaml:"port"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	Resources   ResourcesConfig   `yaml:"resources"`
	Env         map[string]string `yaml:"env"`
}

// HealthCheckConfig defines how health checks should be performed.
type HealthCheckConfig struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
	Retries  int    `yaml:"retries"`
}

// ResourcesConfig defines CPU and memory resource limits.
type ResourcesConfig struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}

// PipelineConfig holds the CI/CD pipeline definition.
type PipelineConfig struct {
	Stages []Stage `yaml:"stages"`
}

// Stage represents a pipeline stage containing one or more steps.
type Stage struct {
	Name  string         `yaml:"name"`
	Steps []PipelineStep `yaml:"steps"`
}

// PipelineStep represents an individual step within a pipeline stage.
type PipelineStep struct {
	Name         string            `yaml:"name"`
	Image        string            `yaml:"image"`
	Commands     []string          `yaml:"commands"`
	Env          map[string]string `yaml:"env"`
	Cache        []string          `yaml:"cache"`
	Artifacts    []string          `yaml:"artifacts"`
	DependsOn    []string          `yaml:"depends_on"`
	Timeout      string            `yaml:"timeout"`
	AllowFailure bool              `yaml:"allow_failure"`
}

// Notifications configures notification channels.
type Notifications struct {
	Slack   SlackConfig   `yaml:"slack"`
	Discord DiscordConfig `yaml:"discord"`
	Email   []string      `yaml:"email"`
}

// SlackConfig holds Slack notification settings.
type SlackConfig struct {
	WebhookURL string `yaml:"webhook_url"`
	Channel    string `yaml:"channel"`
}

// DiscordConfig holds Discord notification settings.
type DiscordConfig struct {
	WebhookURL string `yaml:"webhook_url"`
}

// Parse parses raw YAML bytes into a NexusOpsConfig.
func Parse(data []byte) (*NexusOpsConfig, error) {
	if len(data) == 0 {
		return nil, ErrEmptyConfig
	}

	config := &NexusOpsConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("config: failed to parse YAML: %w", err)
	}

	// Apply defaults
	applyDefaults(config)

	return config, nil
}

// ParseFile reads and parses a nexusops.yaml file from the given path.
func ParseFile(path string) (*NexusOpsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: failed to read file %s: %w", path, err)
	}

	return Parse(data)
}

// applyDefaults sets sensible default values for unspecified fields.
func applyDefaults(config *NexusOpsConfig) {
	if config.Deploy.Provider == "" {
		config.Deploy.Provider = "docker"
	}
	if config.Deploy.Strategy == "" {
		config.Deploy.Strategy = "rolling"
	}
	if config.Deploy.Replicas == 0 {
		config.Deploy.Replicas = 1
	}
	if config.Deploy.Port == 0 {
		config.Deploy.Port = 8080
	}
	if config.Deploy.HealthCheck.Path == "" {
		config.Deploy.HealthCheck.Path = "/health"
	}
	if config.Deploy.HealthCheck.Interval == "" {
		config.Deploy.HealthCheck.Interval = "30s"
	}
	if config.Deploy.HealthCheck.Timeout == "" {
		config.Deploy.HealthCheck.Timeout = "5s"
	}
	if config.Deploy.HealthCheck.Retries == 0 {
		config.Deploy.HealthCheck.Retries = 3
	}
	if config.Build.Dockerfile == "" {
		config.Build.Dockerfile = "Dockerfile"
	}
	if config.Build.Context == "" {
		config.Build.Context = "."
	}
	if config.Deploy.Env == nil {
		config.Deploy.Env = make(map[string]string)
	}
}

// Validate checks the configuration for logical errors and required fields.
func Validate(config *NexusOpsConfig) error {
	var errs []string

	if config.Name == "" {
		errs = append(errs, ErrMissingName.Error())
	}
	if config.Version == "" {
		errs = append(errs, ErrMissingVersion.Error())
	}

	// Validate deploy provider
	if config.Deploy.Provider != "" && !validProviders[config.Deploy.Provider] {
		errs = append(errs, fmt.Sprintf("%s: %q (valid: docker, kubernetes)", ErrInvalidProvider.Error(), config.Deploy.Provider))
	}

	// Validate deploy strategy
	if config.Deploy.Strategy != "" && !validStrategies[config.Deploy.Strategy] {
		errs = append(errs, fmt.Sprintf("%s: %q (valid: rolling, blue-green, canary)", ErrInvalidStrategy.Error(), config.Deploy.Strategy))
	}

	// Validate replicas
	if config.Deploy.Replicas < 1 {
		errs = append(errs, ErrInvalidReplicas.Error())
	}

	// Validate port
	if config.Deploy.Port < 1 || config.Deploy.Port > 65535 {
		errs = append(errs, ErrInvalidPort.Error())
	}

	// Validate health check
	if err := validateHealthCheck(&config.Deploy.HealthCheck); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate pipeline stages
	if err := validatePipeline(&config.Pipeline); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("config: validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// validateHealthCheck verifies the health check configuration is valid.
func validateHealthCheck(hc *HealthCheckConfig) error {
	var errs []string

	if hc.Retries < 1 {
		errs = append(errs, ErrInvalidRetries.Error())
	}

	if hc.Interval != "" {
		if _, err := time.ParseDuration(hc.Interval); err != nil {
			errs = append(errs, fmt.Sprintf("%s: interval %q: %v", ErrInvalidTimeout.Error(), hc.Interval, err))
		}
	}

	if hc.Timeout != "" {
		if _, err := time.ParseDuration(hc.Timeout); err != nil {
			errs = append(errs, fmt.Sprintf("%s: timeout %q: %v", ErrInvalidTimeout.Error(), hc.Timeout, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// validatePipeline validates the pipeline configuration including dependency checks.
func validatePipeline(pipeline *PipelineConfig) error {
	var errs []string

	// Build a global index of all step names across all stages for dependency validation
	allStepNames := make(map[string]bool)
	for _, stage := range pipeline.Stages {
		for _, step := range stage.Steps {
			if step.Name != "" {
				allStepNames[step.Name] = true
			}
		}
	}

	for i, stage := range pipeline.Stages {
		if stage.Name == "" {
			errs = append(errs, fmt.Sprintf("%s at index %d", ErrMissingStageName.Error(), i))
		}

		for j, step := range stage.Steps {
			if step.Name == "" {
				errs = append(errs, fmt.Sprintf("%s at stage %q index %d", ErrMissingStepName.Error(), stage.Name, j))
			}
			if step.Image == "" {
				errs = append(errs, fmt.Sprintf("%s at stage %q step %q", ErrMissingStepImage.Error(), stage.Name, step.Name))
			}

			// Validate timeout if specified
			if step.Timeout != "" {
				if _, err := time.ParseDuration(step.Timeout); err != nil {
					errs = append(errs, fmt.Sprintf("%s: step %q timeout %q: %v", ErrInvalidTimeout.Error(), step.Name, step.Timeout, err))
				}
			}

			// Validate dependencies exist
			for _, dep := range step.DependsOn {
				if !allStepNames[dep] {
					errs = append(errs, fmt.Sprintf("config: step %q depends on unknown step %q", step.Name, dep))
				}
			}
		}
	}

	// Check for circular dependencies across all stages
	if err := checkCircularDependencies(pipeline); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// checkCircularDependencies uses topological sort (DFS) to detect cycles
// in the step dependency graph across all stages.
func checkCircularDependencies(pipeline *PipelineConfig) error {
	// Build dependency graph across all stages
	deps := make(map[string][]string)
	for _, stage := range pipeline.Stages {
		for _, step := range stage.Steps {
			if step.Name != "" {
				deps[step.Name] = step.DependsOn
			}
		}
	}

	// State: 0 = unvisited, 1 = visiting (in current path), 2 = visited
	state := make(map[string]int)

	var visit func(name string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("%w: step %q", ErrCircularDependency, name)
		}
		if state[name] == 2 {
			return nil
		}

		state[name] = 1
		for _, dep := range deps[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		return nil
	}

	for name := range deps {
		if state[name] == 0 {
			if err := visit(name); err != nil {
				return err
			}
		}
	}

	return nil
}
