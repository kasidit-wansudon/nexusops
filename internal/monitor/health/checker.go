// Package health provides a health check engine that periodically probes
// service dependencies and exposes readiness, liveness, and aggregate health
// endpoints.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// Status represents the outcome of a health check.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Result holds the outcome of a single health check execution.
type Result struct {
	Status    Status            `json:"status"`
	Latency   time.Duration     `json:"latency"`
	Message   string            `json:"message,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Details   map[string]string `json:"details,omitempty"`
}

// Check is the interface that all health check implementations must satisfy.
type Check interface {
	Execute(ctx context.Context) *Result
}

// ---------- HTTPCheck ----------

// HTTPCheck performs an HTTP request against a URL and validates the response
// status code.
type HTTPCheck struct {
	URL            string
	Method         string
	ExpectedStatus int
	Timeout        time.Duration
	Headers        map[string]string
}

// Execute sends the configured HTTP request and returns a Result indicating
// whether the response status code matches ExpectedStatus.
func (h *HTTPCheck) Execute(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{
		Timestamp: start,
		Details:   make(map[string]string),
	}

	if h.Method == "" {
		h.Method = http.MethodGet
	}
	if h.ExpectedStatus == 0 {
		h.ExpectedStatus = http.StatusOK
	}
	timeout := h.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, h.Method, h.URL, nil)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Latency = time.Since(start)
		result.Message = fmt.Sprintf("failed to create request: %v", err)
		return result
	}
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	result.Latency = time.Since(start)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.Details["status_code"] = fmt.Sprintf("%d", resp.StatusCode)
	result.Details["url"] = h.URL

	if resp.StatusCode == h.ExpectedStatus {
		result.Status = StatusHealthy
		result.Message = "OK"
	} else {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("unexpected status %d, expected %d", resp.StatusCode, h.ExpectedStatus)
	}
	return result
}

// ---------- TCPCheck ----------

// TCPCheck attempts a TCP connection to an address within the configured timeout.
type TCPCheck struct {
	Address string
	Timeout time.Duration
}

// Execute dials the TCP address and reports whether the connection succeeded.
func (t *TCPCheck) Execute(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{
		Timestamp: start,
		Details:   make(map[string]string),
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	conn, err := net.DialTimeout("tcp", t.Address, timeout)
	result.Latency = time.Since(start)
	result.Details["address"] = t.Address

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("tcp dial failed: %v", err)
		return result
	}
	conn.Close()

	result.Status = StatusHealthy
	result.Message = "connection established"
	return result
}

// ---------- CommandCheck ----------

// CommandCheck executes an external command and treats a zero exit code as healthy.
type CommandCheck struct {
	Command string
	Args    []string
	Timeout time.Duration
}

// Execute runs the command with the given arguments. A non-zero exit code
// results in an unhealthy status.
func (c *CommandCheck) Execute(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{
		Timestamp: start,
		Details:   make(map[string]string),
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.Command, c.Args...)
	output, err := cmd.CombinedOutput()
	result.Latency = time.Since(start)
	result.Details["command"] = c.Command

	if len(output) > 0 {
		// Truncate output for the details map.
		out := string(output)
		if len(out) > 512 {
			out = out[:512] + "..."
		}
		result.Details["output"] = out
	}

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("command failed: %v", err)
		return result
	}

	result.Status = StatusHealthy
	result.Message = "command exited successfully"
	return result
}

// ---------- Checker ----------

// Checker orchestrates periodic execution of registered health checks and
// exposes their results through HTTP handlers.
type Checker struct {
	mu        sync.RWMutex
	checks    map[string]Check
	results   map[string]*Result
	interval  time.Duration
	callbacks []func(name string, result *Result)

	cancel context.CancelFunc
	done   chan struct{}
}

// NewChecker returns a Checker that will run registered checks at the
// specified interval.
func NewChecker(interval time.Duration) *Checker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Checker{
		checks:   make(map[string]Check),
		results:  make(map[string]*Result),
		interval: interval,
	}
}

// Register adds a named health check. If Start has already been called, the
// check will be picked up on the next tick.
func (ch *Checker) Register(name string, check Check) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.checks[name] = check
}

// OnResult registers a callback that is invoked every time a check completes.
func (ch *Checker) OnResult(cb func(name string, result *Result)) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.callbacks = append(ch.callbacks, cb)
}

// Start begins the periodic health check loop. It runs all checks in parallel
// on each tick.
func (ch *Checker) Start(ctx context.Context) {
	ctx, ch.cancel = context.WithCancel(ctx)
	ch.done = make(chan struct{})

	go func() {
		defer close(ch.done)
		// Run immediately on start.
		ch.runAll(ctx)

		ticker := time.NewTicker(ch.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ch.runAll(ctx)
			}
		}
	}()
}

// runAll executes every registered check concurrently and stores results.
func (ch *Checker) runAll(ctx context.Context) {
	ch.mu.RLock()
	names := make([]string, 0, len(ch.checks))
	checks := make([]Check, 0, len(ch.checks))
	for n, c := range ch.checks {
		names = append(names, n)
		checks = append(checks, c)
	}
	ch.mu.RUnlock()

	type pair struct {
		name   string
		result *Result
	}
	results := make(chan pair, len(checks))
	var wg sync.WaitGroup

	for i, c := range checks {
		wg.Add(1)
		go func(name string, check Check) {
			defer wg.Done()
			r := check.Execute(ctx)
			results <- pair{name: name, result: r}
		}(names[i], c)
	}

	wg.Wait()
	close(results)

	ch.mu.Lock()
	cbs := make([]func(string, *Result), len(ch.callbacks))
	copy(cbs, ch.callbacks)
	for p := range results {
		ch.results[p.name] = p.result
	}
	ch.mu.Unlock()

	// Fire callbacks outside the lock.
	ch.mu.RLock()
	for name, res := range ch.results {
		for _, cb := range cbs {
			cb(name, res)
		}
	}
	ch.mu.RUnlock()
}

// Stop cancels the background loop and waits for it to finish.
func (ch *Checker) Stop() {
	if ch.cancel != nil {
		ch.cancel()
	}
	if ch.done != nil {
		<-ch.done
	}
}

// GetStatus returns the most recent result for the named check, or nil if
// the check has not yet run or does not exist.
func (ch *Checker) GetStatus(name string) *Result {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.results[name]
}

// GetAllStatuses returns a copy of every check result.
func (ch *Checker) GetAllStatuses() map[string]*Result {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	out := make(map[string]*Result, len(ch.results))
	for k, v := range ch.results {
		out[k] = v
	}
	return out
}

// IsHealthy returns true only when every registered check has a healthy status.
func (ch *Checker) IsHealthy() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if len(ch.results) == 0 {
		return false
	}
	for _, r := range ch.results {
		if r.Status != StatusHealthy {
			return false
		}
	}
	return true
}

// healthResponse is the JSON structure returned by the health endpoints.
type healthResponse struct {
	Status  Status             `json:"status"`
	Checks  map[string]*Result `json:"checks,omitempty"`
	Elapsed string             `json:"elapsed"`
}

// Handler returns an http.Handler that responds with the aggregate health
// status and individual check results as JSON (the /health endpoint).
func (ch *Checker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		statuses := ch.GetAllStatuses()

		overall := StatusHealthy
		for _, res := range statuses {
			if res.Status == StatusUnhealthy {
				overall = StatusUnhealthy
				break
			}
			if res.Status == StatusDegraded {
				overall = StatusDegraded
			}
		}

		resp := healthResponse{
			Status:  overall,
			Checks:  statuses,
			Elapsed: time.Since(start).String(),
		}

		w.Header().Set("Content-Type", "application/json")
		if overall != StatusHealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	})
}

// ReadinessHandler returns an http.Handler for the /ready endpoint. It returns
// 200 when all checks are healthy and 503 otherwise.
func (ch *Checker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthy := ch.IsHealthy()
		resp := healthResponse{
			Status:  StatusHealthy,
			Elapsed: "0s",
		}
		if !healthy {
			resp.Status = StatusUnhealthy
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
}

// LivenessHandler returns an http.Handler for the /livez endpoint. It always
// returns 200 as long as the process is running, indicating the application
// has not deadlocked.
func (ch *Checker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(healthResponse{
			Status:  StatusHealthy,
			Elapsed: "0s",
		})
	})
}
