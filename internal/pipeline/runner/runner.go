package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/kasidit-wansudon/nexusops/internal/pipeline/artifact"
	logpkg "github.com/kasidit-wansudon/nexusops/internal/pipeline/log"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/parser"
)

// Status constants for step and pipeline results.
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// StepResult captures the outcome of executing a single pipeline step.
type StepResult struct {
	StepName  string        `json:"step_name"`
	Status    string        `json:"status"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	ExitCode  int           `json:"exit_code"`
	LogOutput string        `json:"log_output"`
	Duration  time.Duration `json:"duration"`
}

// PipelineResult captures the outcome of an entire pipeline execution.
type PipelineResult struct {
	PipelineID string        `json:"pipeline_id"`
	Status     string        `json:"status"`
	Steps      []StepResult  `json:"steps"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
}

// Runner orchestrates the execution of pipeline steps inside containers.
type Runner struct {
	dockerClient  client.APIClient
	logStreamer   *logpkg.Streamer
	artifactStore *artifact.Store
	pullPolicy    string
}

// NewRunner creates a Runner with the provided Docker client, log streamer,
// and artifact store.
func NewRunner(dockerClient client.APIClient, logStreamer *logpkg.Streamer, artifactStore *artifact.Store) *Runner {
	return &Runner{
		dockerClient:  dockerClient,
		logStreamer:   logStreamer,
		artifactStore: artifactStore,
		pullPolicy:    "always",
	}
}

// SetPullPolicy controls when images are pulled. Valid values: "always",
// "if-not-present", "never".
func (r *Runner) SetPullPolicy(policy string) {
	r.pullPolicy = policy
}

// ExecutePipeline runs every stage of the given pipeline in dependency order.
// Stages within the same tier may execute concurrently; steps within a stage
// execute in parallel when their Parallel flag is set.
func (r *Runner) ExecutePipeline(ctx context.Context, pipelineID string, pipeline *parser.Pipeline, env map[string]string) (*PipelineResult, error) {
	result := &PipelineResult{
		PipelineID: pipelineID,
		Status:     StatusRunning,
		StartTime:  time.Now(),
		Steps:      make([]StepResult, 0),
	}

	tiers, err := parser.ResolveDependencyOrder(pipeline)
	if err != nil {
		result.Status = StatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("resolve dependency order: %w", err)
	}

	mergedEnv := mergeEnv(nil, env)
	pipelineFailed := false

	for _, tier := range tiers {
		if pipelineFailed {
			for _, stage := range tier {
				for _, step := range stage.Steps {
					result.Steps = append(result.Steps, StepResult{
						StepName: step.Name,
						Status:   StatusSkipped,
					})
				}
			}
			continue
		}

		tierResults, tierErr := r.executeTier(ctx, pipelineID, tier, mergedEnv)
		result.Steps = append(result.Steps, tierResults...)

		if tierErr != nil {
			pipelineFailed = true
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if pipelineFailed {
		result.Status = StatusFailed
	} else {
		result.Status = StatusSuccess
	}

	return result, nil
}

// executeTier runs all stages in a tier. Stages within a tier are independent
// and are executed sequentially here (parallelism could be added if needed).
func (r *Runner) executeTier(ctx context.Context, pipelineID string, stages []parser.Stage, env map[string]string) ([]StepResult, error) {
	var allResults []StepResult
	var tierError error

	for _, stage := range stages {
		stageResults, err := r.executeStage(ctx, pipelineID, stage, env)
		allResults = append(allResults, stageResults...)
		if err != nil {
			tierError = err
		}
	}
	return allResults, tierError
}

// executeStage runs all steps in a stage. When steps have Parallel set they
// are launched concurrently; non-parallel steps run sequentially.
func (r *Runner) executeStage(ctx context.Context, pipelineID string, stage parser.Stage, env map[string]string) ([]StepResult, error) {
	var parallelSteps []parser.Step
	var sequentialSteps []parser.Step

	for _, step := range stage.Steps {
		if step.Parallel {
			parallelSteps = append(parallelSteps, step)
		} else {
			sequentialSteps = append(sequentialSteps, step)
		}
	}

	results := make([]StepResult, 0, len(stage.Steps))
	stageFailed := false

	// Execute parallel steps concurrently.
	if len(parallelSteps) > 0 {
		var mu sync.Mutex
		var wg sync.WaitGroup
		wg.Add(len(parallelSteps))

		for _, step := range parallelSteps {
			go func(s parser.Step) {
				defer wg.Done()
				res, err := r.executeStep(ctx, pipelineID, s, env)
				mu.Lock()
				defer mu.Unlock()
				results = append(results, *res)
				if err != nil && !s.AllowFailure {
					stageFailed = true
				}
			}(step)
		}
		wg.Wait()
	}

	// Execute sequential steps one at a time.
	for _, step := range sequentialSteps {
		if stageFailed {
			results = append(results, StepResult{
				StepName: step.Name,
				Status:   StatusSkipped,
			})
			continue
		}

		res, err := r.executeStep(ctx, pipelineID, step, env)
		results = append(results, *res)
		if err != nil && !step.AllowFailure {
			stageFailed = true
		}
	}

	if stageFailed {
		return results, fmt.Errorf("stage %q failed", stage.Name)
	}
	return results, nil
}

// executeStep runs a single step inside a Docker container. It pulls the
// image, creates a container, injects commands as a shell script, streams
// logs, waits for completion, and collects artifacts.
func (r *Runner) executeStep(ctx context.Context, pipelineID string, step parser.Step, env map[string]string) (*StepResult, error) {
	result := &StepResult{
		StepName:  step.Name,
		Status:    StatusRunning,
		StartTime: time.Now(),
	}

	// Apply step-level timeout.
	stepCtx := ctx
	var cancel context.CancelFunc
	if step.Timeout > 0 {
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Pull the image.
	if err := r.pullImage(stepCtx, step.Image); err != nil {
		result.Status = StatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.LogOutput = fmt.Sprintf("failed to pull image %s: %v", step.Image, err)
		return result, err
	}

	// Build environment variables list.
	stepEnv := mergeEnv(env, step.Env)
	envList := make([]string, 0, len(stepEnv))
	for k, v := range stepEnv {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	// Build the shell script from commands.
	script := buildScript(step.Commands)

	// Create the container.
	containerCfg := &container.Config{
		Image: step.Image,
		Cmd:   []string{"/bin/sh", "-e", "-c", script},
		Env:   envList,
	}

	hostCfg := &container.HostConfig{}

	resp, err := r.dockerClient.ContainerCreate(stepCtx, containerCfg, hostCfg, nil, nil, "")
	if err != nil {
		result.Status = StatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.LogOutput = fmt.Sprintf("failed to create container: %v", err)
		return result, err
	}
	containerID := resp.ID

	// Ensure cleanup.
	defer func() {
		removeCtx, removeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer removeCancel()
		_ = r.dockerClient.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true})
	}()

	// Start the container.
	if err := r.dockerClient.ContainerStart(stepCtx, containerID, container.StartOptions{}); err != nil {
		result.Status = StatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.LogOutput = fmt.Sprintf("failed to start container: %v", err)
		return result, err
	}

	// Stream logs.
	logOutput, logErr := r.streamContainerLogs(stepCtx, pipelineID, step.Name, containerID)

	// Wait for the container to exit.
	statusCh, errCh := r.dockerClient.ContainerWait(stepCtx, containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			result.Status = StatusFailed
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			result.LogOutput = logOutput
			return result, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		result.ExitCode = int(status.StatusCode)
	case <-stepCtx.Done():
		// Timeout — kill the container.
		killCtx, killCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer killCancel()
		_ = r.dockerClient.ContainerKill(killCtx, containerID, "SIGKILL")
		result.Status = StatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.LogOutput = logOutput + "\n[TIMEOUT] step exceeded allowed duration"
		return result, fmt.Errorf("step %q timed out", step.Name)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.LogOutput = logOutput

	if logErr != nil {
		result.LogOutput += fmt.Sprintf("\n[WARNING] log stream error: %v", logErr)
	}

	if result.ExitCode != 0 {
		result.Status = StatusFailed
		if step.AllowFailure {
			return result, nil
		}
		return result, fmt.Errorf("step %q exited with code %d", step.Name, result.ExitCode)
	}

	result.Status = StatusSuccess

	// Collect artifacts if configured.
	if step.Artifacts != nil && r.artifactStore != nil {
		for _, p := range step.Artifacts.Paths {
			_, _ = r.artifactStore.Save(pipelineID, step.Name, p)
		}
	}

	return result, nil
}

// pullImage pulls a container image from a registry.
func (r *Runner) pullImage(ctx context.Context, imageName string) error {
	if r.pullPolicy == "never" {
		return nil
	}

	reader, err := r.dockerClient.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Drain the pull output so the operation completes.
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// streamContainerLogs attaches to container logs and streams them through
// the log streamer. Returns the collected log output.
func (r *Runner) streamContainerLogs(ctx context.Context, pipelineID, stepName, containerID string) (string, error) {
	logsReader, err := r.dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		return "", fmt.Errorf("attach logs: %w", err)
	}
	defer logsReader.Close()

	var buf bytes.Buffer
	tee := io.TeeReader(logsReader, &buf)

	if r.logStreamer != nil {
		r.logStreamer.StreamFromReader(pipelineID, stepName, tee, "stdout")
	} else {
		_, _ = io.Copy(io.Discard, tee)
	}

	return buf.String(), nil
}

// buildScript joins a slice of commands into a single shell script string
// separated by newlines so that the -e flag causes the script to abort on
// the first failing command.
func buildScript(commands []string) string {
	return strings.Join(commands, "\n")
}

// mergeEnv creates a new map from base overlaid with overrides.
func mergeEnv(base, overrides map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}
