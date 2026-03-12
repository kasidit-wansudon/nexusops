package preview

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Environment represents a preview environment spun up for a single PR.
type Environment struct {
	ID          string
	ProjectID   string
	PRNumber    int
	Branch      string
	Subdomain   string
	ContainerID string
	URL         string
	Status      string // "creating", "running", "stopped", "expired", "error"
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// ContainerDeployer is the minimal interface the Manager needs to create and
// destroy containers.  In production it is satisfied by the docker.Deployer.
type ContainerDeployer interface {
	DeployPreview(ctx context.Context, image string, port int, env map[string]string) (containerID string, hostPort int, err error)
	StopPreview(ctx context.Context, containerID string) error
}

// ProxyRouter registers and deregisters subdomain-to-backend routes in the
// edge proxy (e.g. Caddy, Traefik, or an in-process reverse proxy).
type ProxyRouter interface {
	AddRoute(subdomain string, backendAddr string) error
	RemoveRoute(subdomain string) error
}

// Manager creates, tracks, and cleans up preview environments.
type Manager struct {
	mu       sync.RWMutex
	envs     map[string]*Environment // envID -> env
	domain   string                  // e.g. "preview.example.com"
	deployer ContainerDeployer
	router   ProxyRouter
	basePort int
	ttl      time.Duration
}

// NewManager creates a Manager that generates subdomains under the given
// domain (e.g. "preview.example.com") and wires them through the supplied
// deployer and router.
func NewManager(domain string, deployer ContainerDeployer, router ProxyRouter) *Manager {
	return &Manager{
		envs:     make(map[string]*Environment),
		domain:   domain,
		deployer: deployer,
		router:   router,
		basePort: 9000,
		ttl:      72 * time.Hour,
	}
}

// SetTTL configures the default time-to-live for new preview environments.
func (m *Manager) SetTTL(ttl time.Duration) {
	if ttl <= 0 {
		ttl = 72 * time.Hour
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ttl = ttl
}

// Create spins up a preview environment for the given PR, deploys a container
// from the provided image, registers a subdomain route, and returns the
// resulting Environment.
func (m *Manager) Create(ctx context.Context, projectID string, prNumber int, image, branch string) (*Environment, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	if prNumber <= 0 {
		return nil, fmt.Errorf("PR number must be positive, got %d", prNumber)
	}
	if image == "" {
		return nil, fmt.Errorf("image is required")
	}

	// Check for duplicate: only one environment per (project, PR).
	if existing := m.findByPR(projectID, prNumber); existing != nil {
		return nil, fmt.Errorf("preview environment already exists for project %s PR #%d (env %s)", projectID, prNumber, existing.ID)
	}

	envID := uuid.New().String()
	subdomain := buildSubdomain(projectID, prNumber)
	url := fmt.Sprintf("https://%s.%s", subdomain, m.domain)

	env := &Environment{
		ID:        envID,
		ProjectID: projectID,
		PRNumber:  prNumber,
		Branch:    branch,
		Subdomain: subdomain,
		URL:       url,
		Status:    "creating",
		CreatedAt: time.Now().UTC(),
	}

	m.mu.Lock()
	env.ExpiresAt = env.CreatedAt.Add(m.ttl)
	m.mu.Unlock()

	// Deploy the container.
	containerID, hostPort, err := m.deployer.DeployPreview(ctx, image, m.nextPort(), nil)
	if err != nil {
		env.Status = "error"
		m.store(env)
		return env, fmt.Errorf("deploy preview container: %w", err)
	}
	env.ContainerID = containerID

	// Register the route so the subdomain resolves to the container.
	backendAddr := fmt.Sprintf("http://127.0.0.1:%d", hostPort)
	if err := m.router.AddRoute(subdomain+"."+m.domain, backendAddr); err != nil {
		// Best-effort teardown on routing failure.
		_ = m.deployer.StopPreview(ctx, containerID)
		env.Status = "error"
		m.store(env)
		return env, fmt.Errorf("add proxy route: %w", err)
	}

	env.Status = "running"
	m.store(env)
	return env, nil
}

// Delete tears down a preview environment: stops the container and removes
// the proxy route.
func (m *Manager) Delete(ctx context.Context, envID string) error {
	m.mu.Lock()
	env, ok := m.envs[envID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("preview environment %s not found", envID)
	}
	m.mu.Unlock()

	// Stop container.
	if env.ContainerID != "" {
		if err := m.deployer.StopPreview(ctx, env.ContainerID); err != nil {
			return fmt.Errorf("stop preview container %s: %w", env.ContainerID, err)
		}
	}

	// Remove route.
	if err := m.router.RemoveRoute(env.Subdomain + "." + m.domain); err != nil {
		return fmt.Errorf("remove proxy route for %s: %w", env.Subdomain, err)
	}

	m.mu.Lock()
	env.Status = "stopped"
	delete(m.envs, envID)
	m.mu.Unlock()

	return nil
}

// Get returns a copy of the Environment with the given ID.
func (m *Manager) Get(envID string) (*Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	env, ok := m.envs[envID]
	if !ok {
		return nil, fmt.Errorf("preview environment %s not found", envID)
	}

	cp := *env
	return &cp, nil
}

// ListByProject returns all preview environments for the given project.
func (m *Manager) ListByProject(projectID string) []*Environment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Environment
	for _, env := range m.envs {
		if env.ProjectID == projectID {
			cp := *env
			result = append(result, &cp)
		}
	}
	return result
}

// Cleanup removes every preview environment whose ExpiresAt is older than
// time.Now() minus maxAge.  It is safe to call from a periodic goroutine.
func (m *Manager) Cleanup(maxAge time.Duration) (removed int, errs []error) {
	cutoff := time.Now().UTC().Add(-maxAge)

	m.mu.RLock()
	var expired []string
	for id, env := range m.envs {
		if !env.ExpiresAt.IsZero() && env.ExpiresAt.Before(cutoff) {
			expired = append(expired, id)
		}
	}
	m.mu.RUnlock()

	ctx := context.Background()
	for _, id := range expired {
		if err := m.Delete(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("cleanup %s: %w", id, err))
			continue
		}
		removed++
	}

	return removed, errs
}

// RunCleanupLoop runs Cleanup on a fixed interval until the context is
// cancelled.  Typically launched as a goroutine.
func (m *Manager) RunCleanupLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.Cleanup(0)
		}
	}
}

// Count returns the total number of active preview environments.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.envs)
}

// --- internal helpers ---

func (m *Manager) store(env *Environment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.envs[env.ID] = env
}

func (m *Manager) findByPR(projectID string, prNumber int) *Environment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, env := range m.envs {
		if env.ProjectID == projectID && env.PRNumber == prNumber {
			return env
		}
	}
	return nil
}

func (m *Manager) nextPort() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.basePort
	m.basePort++
	return p
}

// buildSubdomain generates a DNS-safe subdomain label for a PR, e.g.
// "pr-42-myproject".
func buildSubdomain(projectID string, prNumber int) string {
	safe := sanitizeDNS(projectID)
	if len(safe) > 40 {
		safe = safe[:40]
	}
	return fmt.Sprintf("pr-%d-%s", prNumber, safe)
}

// sanitizeDNS lowercases the input and replaces characters that are illegal
// in DNS labels with hyphens.
func sanitizeDNS(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
