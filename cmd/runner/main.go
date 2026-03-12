package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/artifact"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/cache"
	plog "github.com/kasidit-wansudon/nexusops/internal/pipeline/log"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/parser"
	"github.com/docker/docker/client"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/runner"
	"github.com/kasidit-wansudon/nexusops/internal/pkg/docker"
)

const (
	version          = "1.0.0"
	defaultPort      = 8081
	maxConcurrentJobs = 5
	jobTimeout       = 30 * time.Minute
)

type RunnerAgent struct {
	id             string
	port           int
	dockerClient   *docker.DockerClient
	pipelineRunner *runner.Runner
	logStreamer    *plog.Streamer
	artifactStore *artifact.Store
	buildCache    *cache.Cache
	jobs          map[string]*Job
	jobQueue      chan *Job
	mu            sync.RWMutex
	wg            sync.WaitGroup
}

type Job struct {
	ID         string            `json:"id"`
	PipelineID string            `json:"pipeline_id"`
	ProjectID  string            `json:"project_id"`
	Config     string            `json:"config"`
	Env        map[string]string `json:"env"`
	Status     string            `json:"status"`
	StartedAt  time.Time         `json:"started_at"`
	EndedAt    *time.Time        `json:"ended_at,omitempty"`
	Result     *runner.PipelineResult `json:"result,omitempty"`
	Error      string            `json:"error,omitempty"`
}

func main() {
	fmt.Printf("NexusOps Pipeline Runner Agent v%s\n", version)

	agent, err := NewRunnerAgent()
	if err != nil {
		log.Fatalf("Failed to create runner agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.StartWorkers(ctx, maxConcurrentJobs)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", agent.healthHandler)
	mux.HandleFunc("/jobs", agent.jobsHandler)
	mux.HandleFunc("/jobs/submit", agent.submitHandler)
	mux.HandleFunc("/jobs/status", agent.statusHandler)
	mux.HandleFunc("/jobs/cancel", agent.cancelHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", agent.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("Runner agent %s listening on :%d", agent.id, agent.port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Runner server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Runner agent shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Runner shutdown error: %v", err)
	}

	agent.wg.Wait()
	log.Println("Runner agent stopped")
}

func NewRunnerAgent() (*RunnerAgent, error) {
	dockerClient, err := docker.NewDockerClient()
	if err != nil {
		log.Printf("Warning: Docker not available: %v", err)
	}

	logStreamer := plog.NewStreamer(50000)
	artifactStore, err := artifact.NewStore("/tmp/nexusops/runner/artifacts")
	if err != nil {
		return nil, fmt.Errorf("create artifact store: %w", err)
	}
	buildCache, err := cache.NewCache("/tmp/nexusops/runner/cache", 5<<30)
	if err != nil {
		return nil, fmt.Errorf("create build cache: %w", err)
	}
	var apiClient client.APIClient
	if dockerClient != nil {
		apiClient = dockerClient.APIClient()
	}
	pipelineRunner := runner.NewRunner(apiClient, logStreamer, artifactStore)

	port := defaultPort
	if p := os.Getenv("RUNNER_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	return &RunnerAgent{
		id:             fmt.Sprintf("runner-%s", uuid.New().String()[:8]),
		port:           port,
		dockerClient:   dockerClient,
		pipelineRunner: pipelineRunner,
		logStreamer:    logStreamer,
		artifactStore: artifactStore,
		buildCache:    buildCache,
		jobs:          make(map[string]*Job),
		jobQueue:      make(chan *Job, 100),
	}, nil
}

func (a *RunnerAgent) StartWorkers(ctx context.Context, count int) {
	for i := 0; i < count; i++ {
		a.wg.Add(1)
		go a.worker(ctx, i)
	}
	log.Printf("Started %d pipeline workers", count)
}

func (a *RunnerAgent) worker(ctx context.Context, id int) {
	defer a.wg.Done()
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping", id)
			return
		case job := <-a.jobQueue:
			a.executeJob(ctx, job)
		}
	}
}

func (a *RunnerAgent) executeJob(ctx context.Context, job *Job) {
	log.Printf("Executing job %s for pipeline %s", job.ID, job.PipelineID)

	a.updateJobStatus(job.ID, "running")

	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	pipeline, err := parser.ParsePipeline([]byte(job.Config))
	if err != nil {
		a.failJob(job.ID, fmt.Sprintf("pipeline parse error: %v", err))
		return
	}

	if errs := parser.Validate(pipeline); len(errs) > 0 {
		a.failJob(job.ID, fmt.Sprintf("pipeline validation errors: %v", errs))
		return
	}

	result, err := a.pipelineRunner.ExecutePipeline(jobCtx, job.PipelineID, pipeline, job.Env)
	if err != nil {
		a.failJob(job.ID, fmt.Sprintf("execution error: %v", err))
		return
	}

	a.mu.Lock()
	if j, ok := a.jobs[job.ID]; ok {
		now := time.Now()
		j.Status = string(result.Status)
		j.EndedAt = &now
		j.Result = result
	}
	a.mu.Unlock()

	log.Printf("Job %s completed with status: %s", job.ID, result.Status)
}

func (a *RunnerAgent) updateJobStatus(jobID, status string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if j, ok := a.jobs[jobID]; ok {
		j.Status = status
	}
}

func (a *RunnerAgent) failJob(jobID, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if j, ok := a.jobs[jobID]; ok {
		now := time.Now()
		j.Status = "failed"
		j.EndedAt = &now
		j.Error = errMsg
	}
	log.Printf("Job %s failed: %s", jobID, errMsg)
}

func (a *RunnerAgent) healthHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	activeJobs := 0
	for _, j := range a.jobs {
		if j.Status == "running" {
			activeJobs++
		}
	}
	a.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"runner_id":   a.id,
		"active_jobs": activeJobs,
		"max_jobs":    maxConcurrentJobs,
		"version":     version,
	})
}

func (a *RunnerAgent) submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PipelineID string            `json:"pipeline_id"`
		ProjectID  string            `json:"project_id"`
		Config     string            `json:"config"`
		Env        map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	job := &Job{
		ID:         uuid.New().String(),
		PipelineID: req.PipelineID,
		ProjectID:  req.ProjectID,
		Config:     req.Config,
		Env:        req.Env,
		Status:     "queued",
		StartedAt:  time.Now(),
	}

	a.mu.Lock()
	a.jobs[job.ID] = job
	a.mu.Unlock()

	select {
	case a.jobQueue <- job:
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"job_id": job.ID,
			"status": "queued",
		})
	default:
		a.mu.Lock()
		delete(a.jobs, job.ID)
		a.mu.Unlock()
		http.Error(w, "job queue full", http.StatusServiceUnavailable)
	}
}

func (a *RunnerAgent) jobsHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	jobs := make([]*Job, 0, len(a.jobs))
	for _, j := range a.jobs {
		jobs = append(jobs, j)
	}
	json.NewEncoder(w).Encode(jobs)
}

func (a *RunnerAgent) statusHandler(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	job, ok := a.jobs[jobID]
	a.mu.RUnlock()

	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(job)
}

func (a *RunnerAgent) cancelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	job, ok := a.jobs[jobID]
	if ok && job.Status == "running" || job.Status == "queued" {
		now := time.Now()
		job.Status = "cancelled"
		job.EndedAt = &now
	}
	a.mu.Unlock()

	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}
