package parser

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Pipeline represents a complete CI/CD pipeline configuration parsed from YAML.
type Pipeline struct {
	Name    string  `yaml:"name"`
	Trigger Trigger `yaml:"trigger"`
	Stages  []Stage `yaml:"stages"`
}

// Trigger defines when the pipeline should execute.
type Trigger struct {
	Branches []string `yaml:"branches"`
	Events   []string `yaml:"events"`
}

// Stage represents a group of steps that execute together.
type Stage struct {
	Name      string   `yaml:"name"`
	Steps     []Step   `yaml:"steps"`
	DependsOn []string `yaml:"depends_on"`
}

// Step represents a single unit of work within a stage.
type Step struct {
	Name         string            `yaml:"name"`
	Image        string            `yaml:"image"`
	Commands     []string          `yaml:"commands"`
	Env          map[string]string `yaml:"env"`
	Cache        *CacheConfig      `yaml:"cache,omitempty"`
	Artifacts    *ArtifactConfig   `yaml:"artifacts,omitempty"`
	Timeout      time.Duration     `yaml:"timeout"`
	AllowFailure bool             `yaml:"allow_failure"`
	Parallel     bool             `yaml:"parallel"`
	Services     []Service         `yaml:"services"`
}

// Service represents a sidecar service for a step (e.g., a database).
type Service struct {
	Name  string            `yaml:"name"`
	Image string            `yaml:"image"`
	Env   map[string]string `yaml:"env"`
}

// CacheConfig defines caching behaviour for a step.
type CacheConfig struct {
	Key   string   `yaml:"key"`
	Paths []string `yaml:"paths"`
}

// ArtifactConfig defines which files to preserve after a step completes.
type ArtifactConfig struct {
	Paths    []string `yaml:"paths"`
	ExpireIn string   `yaml:"expire_in"`
}

// stepTimeout is a helper type for unmarshalling durations given as strings.
type stepTimeout struct {
	Name         string            `yaml:"name"`
	Image        string            `yaml:"image"`
	Commands     []string          `yaml:"commands"`
	Env          map[string]string `yaml:"env"`
	Cache        *CacheConfig      `yaml:"cache,omitempty"`
	Artifacts    *ArtifactConfig   `yaml:"artifacts,omitempty"`
	Timeout      string            `yaml:"timeout"`
	AllowFailure bool             `yaml:"allow_failure"`
	Parallel     bool             `yaml:"parallel"`
	Services     []Service         `yaml:"services"`
}

// rawPipeline mirrors Pipeline but uses stepTimeout for flexible duration parsing.
type rawPipeline struct {
	Name    string  `yaml:"name"`
	Trigger Trigger `yaml:"trigger"`
	Stages  []struct {
		Name      string       `yaml:"name"`
		Steps     []stepTimeout `yaml:"steps"`
		DependsOn []string     `yaml:"depends_on"`
	} `yaml:"stages"`
}

// ParsePipeline parses raw YAML bytes into a Pipeline struct. It handles
// duration strings such as "30s", "5m", or "1h" for step timeouts.
func ParsePipeline(data []byte) (*Pipeline, error) {
	var raw rawPipeline
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}

	pipeline := &Pipeline{
		Name:    raw.Name,
		Trigger: raw.Trigger,
		Stages:  make([]Stage, 0, len(raw.Stages)),
	}

	for i, rs := range raw.Stages {
		stage := Stage{
			Name:      rs.Name,
			DependsOn: rs.DependsOn,
			Steps:     make([]Step, 0, len(rs.Steps)),
		}

		for j, rawStep := range rs.Steps {
			var timeout time.Duration
			if rawStep.Timeout != "" {
				d, err := time.ParseDuration(rawStep.Timeout)
				if err != nil {
					return nil, fmt.Errorf("stage %d step %d: invalid timeout %q: %w", i, j, rawStep.Timeout, err)
				}
				timeout = d
			}

			step := Step{
				Name:         rawStep.Name,
				Image:        rawStep.Image,
				Commands:     rawStep.Commands,
				Env:          rawStep.Env,
				Cache:        rawStep.Cache,
				Artifacts:    rawStep.Artifacts,
				Timeout:      timeout,
				AllowFailure: rawStep.AllowFailure,
				Parallel:     rawStep.Parallel,
				Services:     rawStep.Services,
			}
			stage.Steps = append(stage.Steps, step)
		}
		pipeline.Stages = append(pipeline.Stages, stage)
	}

	return pipeline, nil
}

// ParseFile reads a YAML file from disk and parses it into a Pipeline.
func ParseFile(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pipeline file %s: %w", path, err)
	}
	return ParsePipeline(data)
}

// Validate checks a pipeline for structural correctness and returns all
// discovered errors. It verifies required fields, dependency references,
// and ensures the dependency graph is acyclic.
func Validate(p *Pipeline) []error {
	var errs []error

	if p.Name == "" {
		errs = append(errs, fmt.Errorf("pipeline name is required"))
	}

	if len(p.Stages) == 0 {
		errs = append(errs, fmt.Errorf("pipeline must have at least one stage"))
	}

	stageNames := make(map[string]bool, len(p.Stages))
	for _, s := range p.Stages {
		if s.Name == "" {
			errs = append(errs, fmt.Errorf("stage name is required"))
			continue
		}
		if stageNames[s.Name] {
			errs = append(errs, fmt.Errorf("duplicate stage name: %s", s.Name))
		}
		stageNames[s.Name] = true
	}

	// Validate dependency references exist.
	for _, s := range p.Stages {
		for _, dep := range s.DependsOn {
			if !stageNames[dep] {
				errs = append(errs, fmt.Errorf("stage %q depends on unknown stage %q", s.Name, dep))
			}
			if dep == s.Name {
				errs = append(errs, fmt.Errorf("stage %q cannot depend on itself", s.Name))
			}
		}
	}

	// Check for cycles using topological sort.
	if cycleErr := detectCycles(p.Stages); cycleErr != nil {
		errs = append(errs, cycleErr)
	}

	// Validate steps within each stage.
	for _, s := range p.Stages {
		if len(s.Steps) == 0 {
			errs = append(errs, fmt.Errorf("stage %q must have at least one step", s.Name))
		}
		stepNames := make(map[string]bool, len(s.Steps))
		for _, step := range s.Steps {
			if step.Name == "" {
				errs = append(errs, fmt.Errorf("step name is required in stage %q", s.Name))
				continue
			}
			if stepNames[step.Name] {
				errs = append(errs, fmt.Errorf("duplicate step name %q in stage %q", step.Name, s.Name))
			}
			stepNames[step.Name] = true

			if step.Image == "" {
				errs = append(errs, fmt.Errorf("step %q in stage %q requires an image", step.Name, s.Name))
			}
			if len(step.Commands) == 0 {
				errs = append(errs, fmt.Errorf("step %q in stage %q requires at least one command", step.Name, s.Name))
			}

			if step.Cache != nil {
				if step.Cache.Key == "" {
					errs = append(errs, fmt.Errorf("step %q cache requires a key", step.Name))
				}
				if len(step.Cache.Paths) == 0 {
					errs = append(errs, fmt.Errorf("step %q cache requires at least one path", step.Name))
				}
			}

			if step.Artifacts != nil && len(step.Artifacts.Paths) == 0 {
				errs = append(errs, fmt.Errorf("step %q artifacts requires at least one path", step.Name))
			}

			for si, svc := range step.Services {
				if svc.Name == "" {
					errs = append(errs, fmt.Errorf("service %d in step %q requires a name", si, step.Name))
				}
				if svc.Image == "" {
					errs = append(errs, fmt.Errorf("service %q in step %q requires an image", svc.Name, step.Name))
				}
			}
		}
	}

	return errs
}

// detectCycles uses Kahn's algorithm to verify the stage dependency graph is
// a DAG. Returns an error if a cycle is detected.
func detectCycles(stages []Stage) error {
	inDegree := make(map[string]int, len(stages))
	adjacency := make(map[string][]string, len(stages))

	for _, s := range stages {
		if _, ok := inDegree[s.Name]; !ok {
			inDegree[s.Name] = 0
		}
		for _, dep := range s.DependsOn {
			adjacency[dep] = append(adjacency[dep], s.Name)
			inDegree[s.Name]++
		}
	}

	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	// Sort for deterministic ordering.
	sort.Strings(queue)

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		neighbors := adjacency[node]
		sort.Strings(neighbors)
		for _, n := range neighbors {
			inDegree[n]--
			if inDegree[n] == 0 {
				queue = append(queue, n)
			}
		}
	}

	if visited != len(stages) {
		// Collect the stages that are part of the cycle.
		var cycleMembers []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycleMembers = append(cycleMembers, name)
			}
		}
		sort.Strings(cycleMembers)
		return fmt.Errorf("dependency cycle detected among stages: %s", strings.Join(cycleMembers, ", "))
	}

	return nil
}

// ResolveDependencyOrder performs a topological sort on the pipeline stages
// and returns them grouped into tiers. Each tier contains stages whose
// dependencies have all been satisfied by prior tiers, meaning the stages
// within a single tier can execute concurrently.
func ResolveDependencyOrder(p *Pipeline) ([][]Stage, error) {
	if err := detectCycles(p.Stages); err != nil {
		return nil, err
	}

	stageMap := make(map[string]Stage, len(p.Stages))
	inDegree := make(map[string]int, len(p.Stages))
	adjacency := make(map[string][]string, len(p.Stages))

	for _, s := range p.Stages {
		stageMap[s.Name] = s
		if _, ok := inDegree[s.Name]; !ok {
			inDegree[s.Name] = 0
		}
		for _, dep := range s.DependsOn {
			adjacency[dep] = append(adjacency[dep], s.Name)
			inDegree[s.Name]++
		}
	}

	// Gather the initial tier — stages with no dependencies.
	currentTier := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			currentTier = append(currentTier, name)
		}
	}
	sort.Strings(currentTier)

	var result [][]Stage

	for len(currentTier) > 0 {
		tier := make([]Stage, 0, len(currentTier))
		nextTier := make([]string, 0)

		for _, name := range currentTier {
			tier = append(tier, stageMap[name])

			neighbors := adjacency[name]
			sort.Strings(neighbors)
			for _, n := range neighbors {
				inDegree[n]--
				if inDegree[n] == 0 {
					nextTier = append(nextTier, n)
				}
			}
		}

		result = append(result, tier)
		sort.Strings(nextTier)
		currentTier = nextTier
	}

	return result, nil
}
