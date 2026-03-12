package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePipelineBasic(t *testing.T) {
	yaml := `
name: test-pipeline
trigger:
  branches:
    - main
    - develop
  events:
    - push
stages:
  - name: build
    steps:
      - name: compile
        image: golang:1.22
        commands:
          - go build ./...
`
	p, err := ParsePipeline([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, "test-pipeline", p.Name)
	assert.Equal(t, []string{"main", "develop"}, p.Trigger.Branches)
	assert.Equal(t, []string{"push"}, p.Trigger.Events)
	require.Len(t, p.Stages, 1)
	assert.Equal(t, "build", p.Stages[0].Name)
	require.Len(t, p.Stages[0].Steps, 1)
	assert.Equal(t, "compile", p.Stages[0].Steps[0].Name)
	assert.Equal(t, "golang:1.22", p.Stages[0].Steps[0].Image)
	assert.Equal(t, []string{"go build ./..."}, p.Stages[0].Steps[0].Commands)
}

func TestParsePipelineWithTimeout(t *testing.T) {
	yaml := `
name: timeout-pipeline
stages:
  - name: test
    steps:
      - name: run-tests
        image: golang:1.22
        commands:
          - go test ./...
        timeout: 30s
`
	p, err := ParsePipeline([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, p.Stages[0].Steps[0].Timeout)
}

func TestParsePipelineInvalidTimeout(t *testing.T) {
	yaml := `
name: bad-timeout
stages:
  - name: test
    steps:
      - name: run
        image: alpine
        commands:
          - echo hello
        timeout: notaduration
`
	_, err := ParsePipeline([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout")
}

func TestParsePipelineInvalidYAML(t *testing.T) {
	_, err := ParsePipeline([]byte(`{{{not valid yaml`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "yaml parse error")
}

func TestParsePipelineWithDependencies(t *testing.T) {
	yaml := `
name: dep-pipeline
stages:
  - name: build
    steps:
      - name: compile
        image: golang:1.22
        commands:
          - go build
  - name: test
    depends_on:
      - build
    steps:
      - name: unit-test
        image: golang:1.22
        commands:
          - go test ./...
  - name: deploy
    depends_on:
      - test
    steps:
      - name: push
        image: docker
        commands:
          - docker push myapp
`
	p, err := ParsePipeline([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, p.Stages, 3)
	assert.Empty(t, p.Stages[0].DependsOn)
	assert.Equal(t, []string{"build"}, p.Stages[1].DependsOn)
	assert.Equal(t, []string{"test"}, p.Stages[2].DependsOn)
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
name: file-pipeline
stages:
  - name: lint
    steps:
      - name: golangci-lint
        image: golangci/golangci-lint
        commands:
          - golangci-lint run
`
	path := filepath.Join(dir, "pipeline.yaml")
	err := os.WriteFile(path, []byte(yamlContent), 0644)
	require.NoError(t, err)

	p, err := ParseFile(path)
	require.NoError(t, err)
	assert.Equal(t, "file-pipeline", p.Name)
	require.Len(t, p.Stages, 1)
	assert.Equal(t, "lint", p.Stages[0].Name)
}

func TestParseFileNotExist(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/pipeline.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read pipeline file")
}

func TestValidateValidPipeline(t *testing.T) {
	p := &Pipeline{
		Name: "valid-pipeline",
		Stages: []Stage{
			{
				Name: "build",
				Steps: []Step{
					{Name: "compile", Image: "golang:1.22", Commands: []string{"go build"}},
				},
			},
		},
	}
	errs := Validate(p)
	assert.Empty(t, errs)
}

func TestValidateMissingFields(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *Pipeline
		errCount int
		errMsg   string
	}{
		{
			name:     "missing pipeline name",
			pipeline: &Pipeline{Stages: []Stage{{Name: "s", Steps: []Step{{Name: "st", Image: "img", Commands: []string{"cmd"}}}}}},
			errCount: 1,
			errMsg:   "pipeline name is required",
		},
		{
			name:     "no stages",
			pipeline: &Pipeline{Name: "p"},
			errCount: 1,
			errMsg:   "pipeline must have at least one stage",
		},
		{
			name: "missing step image",
			pipeline: &Pipeline{
				Name: "p",
				Stages: []Stage{
					{Name: "s", Steps: []Step{{Name: "st", Commands: []string{"cmd"}}}},
				},
			},
			errCount: 1,
			errMsg:   "requires an image",
		},
		{
			name: "missing step commands",
			pipeline: &Pipeline{
				Name: "p",
				Stages: []Stage{
					{Name: "s", Steps: []Step{{Name: "st", Image: "img"}}},
				},
			},
			errCount: 1,
			errMsg:   "requires at least one command",
		},
		{
			name: "empty stage name",
			pipeline: &Pipeline{
				Name: "p",
				Stages: []Stage{
					{Name: "", Steps: []Step{{Name: "st", Image: "img", Commands: []string{"cmd"}}}},
				},
			},
			errCount: 1,
			errMsg:   "stage name is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := Validate(tc.pipeline)
			assert.GreaterOrEqual(t, len(errs), tc.errCount, "expected at least %d errors, got %d", tc.errCount, len(errs))
			found := false
			for _, e := range errs {
				if assert.Error(t, e) && contains(e.Error(), tc.errMsg) {
					found = true
				}
			}
			assert.True(t, found, "expected error containing %q in %v", tc.errMsg, errs)
		})
	}
}

func TestValidateDuplicateStages(t *testing.T) {
	p := &Pipeline{
		Name: "dup-pipeline",
		Stages: []Stage{
			{Name: "build", Steps: []Step{{Name: "s1", Image: "img", Commands: []string{"cmd"}}}},
			{Name: "build", Steps: []Step{{Name: "s2", Image: "img", Commands: []string{"cmd"}}}},
		},
	}
	errs := Validate(p)
	found := false
	for _, e := range errs {
		if contains(e.Error(), "duplicate stage name") {
			found = true
		}
	}
	assert.True(t, found, "expected duplicate stage name error")
}

func TestValidateCyclicDependencies(t *testing.T) {
	p := &Pipeline{
		Name: "cyclic-pipeline",
		Stages: []Stage{
			{Name: "a", DependsOn: []string{"c"}, Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
			{Name: "b", DependsOn: []string{"a"}, Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
			{Name: "c", DependsOn: []string{"b"}, Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
		},
	}
	errs := Validate(p)
	found := false
	for _, e := range errs {
		if contains(e.Error(), "dependency cycle") {
			found = true
		}
	}
	assert.True(t, found, "expected dependency cycle error")
}

func TestResolveDependencyOrder(t *testing.T) {
	p := &Pipeline{
		Name: "ordered-pipeline",
		Stages: []Stage{
			{Name: "build", Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
			{Name: "test", DependsOn: []string{"build"}, Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
			{Name: "deploy", DependsOn: []string{"test"}, Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
		},
	}

	tiers, err := ResolveDependencyOrder(p)
	require.NoError(t, err)
	require.Len(t, tiers, 3)
	assert.Equal(t, "build", tiers[0][0].Name)
	assert.Equal(t, "test", tiers[1][0].Name)
	assert.Equal(t, "deploy", tiers[2][0].Name)
}

func TestResolveDependencyOrderParallel(t *testing.T) {
	p := &Pipeline{
		Name: "parallel-pipeline",
		Stages: []Stage{
			{Name: "build-a", Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
			{Name: "build-b", Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
			{Name: "deploy", DependsOn: []string{"build-a", "build-b"}, Steps: []Step{{Name: "s", Image: "img", Commands: []string{"cmd"}}}},
		},
	}

	tiers, err := ResolveDependencyOrder(p)
	require.NoError(t, err)
	require.Len(t, tiers, 2)
	// First tier should have both build stages (sorted alphabetically)
	assert.Len(t, tiers[0], 2)
	assert.Equal(t, "build-a", tiers[0][0].Name)
	assert.Equal(t, "build-b", tiers[0][1].Name)
	// Second tier should have the deploy stage
	assert.Len(t, tiers[1], 1)
	assert.Equal(t, "deploy", tiers[1][0].Name)
}

func TestParsePipelineWithServices(t *testing.T) {
	yaml := `
name: service-pipeline
stages:
  - name: integration
    steps:
      - name: test-with-db
        image: golang:1.22
        commands:
          - go test -tags=integration ./...
        services:
          - name: postgres
            image: postgres:15
            env:
              POSTGRES_PASSWORD: test
`
	p, err := ParsePipeline([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, p.Stages[0].Steps[0].Services, 1)
	svc := p.Stages[0].Steps[0].Services[0]
	assert.Equal(t, "postgres", svc.Name)
	assert.Equal(t, "postgres:15", svc.Image)
	assert.Equal(t, "test", svc.Env["POSTGRES_PASSWORD"])
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
