package rollback

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// DeploymentRecord captures everything needed to reproduce or roll back a
// particular deployment version.
type DeploymentRecord struct {
	ID        string
	ProjectID string
	Version   string
	Image     string
	Config    map[string]string // arbitrary key-value config carried forward
	Status    string            // "success", "failed", "rolling-back"
	Timestamp time.Time
}

// redeployFunc is the signature the Manager calls when it needs to re-deploy
// a previous version.  Callers inject their real deployer at construction time.
type redeployFunc func(ctx context.Context, record *DeploymentRecord) error

// Manager keeps an ordered history of deployments per project and provides
// manual and automatic rollback capabilities.
type Manager struct {
	mu                  sync.RWMutex
	history             map[string][]*DeploymentRecord // projectID -> records (newest last)
	healthCheckInterval time.Duration
	failureThreshold    int
	redeploy            redeployFunc
	httpClient          *http.Client
}

// NewManager creates a Manager.  healthCheckInterval controls how often the
// auto-rollback loop probes the health endpoint.  If it is zero a default of
// 10 seconds is used.
func NewManager(healthCheckInterval time.Duration) *Manager {
	if healthCheckInterval <= 0 {
		healthCheckInterval = 10 * time.Second
	}
	return &Manager{
		history:             make(map[string][]*DeploymentRecord),
		healthCheckInterval: healthCheckInterval,
		failureThreshold:    3,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SetRedeployFunc allows callers to inject the function used to redeploy a
// previous version.  It must be called before any rollback is triggered.
func (m *Manager) SetRedeployFunc(fn redeployFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.redeploy = fn
}

// SetFailureThreshold sets how many consecutive health-check failures trigger
// an automatic rollback.  Default is 3.
func (m *Manager) SetFailureThreshold(n int) {
	if n < 1 {
		n = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureThreshold = n
}

// RecordDeployment appends a deployment record to the project's history.
func (m *Manager) RecordDeployment(record *DeploymentRecord) {
	if record == nil {
		return
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.history[record.ProjectID] = append(m.history[record.ProjectID], record)
}

// GetPreviousVersion returns the most recent successful deployment that is
// not the current (latest) record.
func (m *Manager) GetPreviousVersion(projectID string) (*DeploymentRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records, ok := m.history[projectID]
	if !ok || len(records) == 0 {
		return nil, fmt.Errorf("no deployment history for project %s", projectID)
	}

	// Walk backwards from the second-to-last entry.
	for i := len(records) - 2; i >= 0; i-- {
		if records[i].Status == "success" {
			cp := *records[i]
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("no previous successful deployment found for project %s", projectID)
}

// Rollback finds the previous successful deployment and redeploys it.  The
// new deployment is recorded in the history with a "success" status on
// completion.
func (m *Manager) Rollback(ctx context.Context, projectID string) (*DeploymentRecord, error) {
	prev, err := m.GetPreviousVersion(projectID)
	if err != nil {
		return nil, fmt.Errorf("rollback: %w", err)
	}

	m.mu.RLock()
	fn := m.redeploy
	m.mu.RUnlock()

	if fn == nil {
		return nil, fmt.Errorf("rollback: redeploy function not set")
	}

	// Mark the current deployment as failed.
	m.markLatest(projectID, "failed")

	// Re-deploy the previous version.
	if err := fn(ctx, prev); err != nil {
		return nil, fmt.Errorf("rollback redeploy: %w", err)
	}

	// Record the rollback as a new successful deployment.
	rolled := &DeploymentRecord{
		ID:        prev.ID + "-rollback",
		ProjectID: projectID,
		Version:   prev.Version,
		Image:     prev.Image,
		Config:    prev.Config,
		Status:    "success",
		Timestamp: time.Now().UTC(),
	}
	m.RecordDeployment(rolled)

	return rolled, nil
}

// AutoRollback monitors the given healthCheckURL at the configured interval.
// When the number of consecutive failures reaches the threshold it triggers a
// rollback automatically.  The function blocks until the context is cancelled
// or a rollback is performed.  It returns nil when a rollback succeeds or the
// context is done.
func (m *Manager) AutoRollback(ctx context.Context, projectID string, healthCheckURL string) error {
	if healthCheckURL == "" {
		return fmt.Errorf("health check URL must not be empty")
	}

	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			healthy := m.probe(ctx, healthCheckURL)
			if healthy {
				consecutiveFailures = 0
				continue
			}

			consecutiveFailures++

			m.mu.RLock()
			threshold := m.failureThreshold
			m.mu.RUnlock()

			if consecutiveFailures >= threshold {
				_, err := m.Rollback(ctx, projectID)
				if err != nil {
					return fmt.Errorf("auto-rollback failed: %w", err)
				}
				return nil // rollback succeeded
			}
		}
	}
}

// GetHistory returns the full deployment history for a project, ordered from
// oldest to newest.
func (m *Manager) GetHistory(projectID string) []*DeploymentRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.history[projectID]
	if len(records) == 0 {
		return nil
	}

	// Return copies so callers cannot mutate internal state.
	out := make([]*DeploymentRecord, len(records))
	for i, r := range records {
		cp := *r
		out[i] = &cp
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})

	return out
}

// GetLatest returns the most recent deployment record for a project.
func (m *Manager) GetLatest(projectID string) (*DeploymentRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.history[projectID]
	if len(records) == 0 {
		return nil, fmt.Errorf("no deployment history for project %s", projectID)
	}

	cp := *records[len(records)-1]
	return &cp, nil
}

// ClearHistory removes all deployment records for a project.
func (m *Manager) ClearHistory(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.history, projectID)
}

// --- internal helpers ---

// markLatest sets the status of the most recent record for a project.
func (m *Manager) markLatest(projectID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	records := m.history[projectID]
	if len(records) > 0 {
		records[len(records)-1].Status = status
	}
}

// probe performs a single HTTP GET against the URL and returns true when the
// response is 2xx.
func (m *Manager) probe(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
