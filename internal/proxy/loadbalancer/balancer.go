package loadbalancer

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Backend represents a single backend server in a load balancer pool.
type Backend struct {
	Address           string    `json:"address"`
	Weight            int       `json:"weight"`
	Healthy           bool      `json:"healthy"`
	ActiveConnections int64     `json:"active_connections"`
	LastCheck         time.Time `json:"last_check"`
}

// Balancer defines the interface for load balancing strategies.
type Balancer interface {
	// Next returns the next healthy backend according to the balancing strategy.
	Next() (*Backend, error)
	// AddBackend adds a backend server to the pool.
	AddBackend(b *Backend)
	// RemoveBackend removes a backend by its address.
	RemoveBackend(address string)
	// HealthCheck performs a health check on all backends.
	HealthCheck(ctx context.Context)
}

// errNoBackends is returned when no backends are available.
var errNoBackends = fmt.Errorf("no backends available")

// errNoHealthyBackends is returned when all backends are unhealthy.
var errNoHealthyBackends = fmt.Errorf("no healthy backends available")

// --- RoundRobin ---

// RoundRobin implements a simple round-robin load balancing strategy that
// cycles through healthy backends sequentially.
type RoundRobin struct {
	backends []*Backend
	current  atomic.Uint64
	mu       sync.RWMutex
}

// NewRoundRobin creates a new round-robin balancer.
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{
		backends: make([]*Backend, 0),
	}
}

// Next returns the next healthy backend in round-robin order.
func (rr *RoundRobin) Next() (*Backend, error) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if len(rr.backends) == 0 {
		return nil, errNoBackends
	}

	n := uint64(len(rr.backends))
	// Try all backends starting from current position
	for i := uint64(0); i < n; i++ {
		idx := rr.current.Add(1) % n
		backend := rr.backends[idx]
		if backend.Healthy {
			return backend, nil
		}
	}

	return nil, errNoHealthyBackends
}

// AddBackend adds a backend to the round-robin pool.
func (rr *RoundRobin) AddBackend(b *Backend) {
	if b.Weight == 0 {
		b.Weight = 1
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.backends = append(rr.backends, b)
}

// RemoveBackend removes a backend by address from the round-robin pool.
func (rr *RoundRobin) RemoveBackend(address string) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	for i, b := range rr.backends {
		if b.Address == address {
			rr.backends = append(rr.backends[:i], rr.backends[i+1:]...)
			return
		}
	}
}

// HealthCheck performs TCP health checks on all backends, marking them
// healthy or unhealthy based on connection success.
func (rr *RoundRobin) HealthCheck(ctx context.Context) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	performHealthChecks(ctx, rr.backends)
}

// --- Weighted (Smooth Weighted Round-Robin) ---

// weightedEntry tracks the current and effective weights for smooth
// weighted round-robin selection.
type weightedEntry struct {
	backend         *Backend
	effectiveWeight int
	currentWeight   int
}

// Weighted implements smooth weighted round-robin load balancing. Backends with
// higher weights receive proportionally more traffic, distributed evenly over
// time to avoid burst allocation.
type Weighted struct {
	entries []*weightedEntry
	mu      sync.RWMutex
}

// NewWeighted creates a new weighted round-robin balancer.
func NewWeighted() *Weighted {
	return &Weighted{
		entries: make([]*weightedEntry, 0),
	}
}

// Next selects the next backend using the smooth weighted round-robin algorithm.
// Each call adjusts current weights to ensure proportional and smooth distribution.
func (w *Weighted) Next() (*Backend, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.entries) == 0 {
		return nil, errNoBackends
	}

	// Collect only healthy entries
	healthy := make([]*weightedEntry, 0, len(w.entries))
	for _, e := range w.entries {
		if e.backend.Healthy {
			healthy = append(healthy, e)
		}
	}
	if len(healthy) == 0 {
		return nil, errNoHealthyBackends
	}

	// Smooth weighted round-robin: Nginx-style algorithm
	totalWeight := 0
	var best *weightedEntry

	for _, e := range healthy {
		e.currentWeight += e.effectiveWeight
		totalWeight += e.effectiveWeight

		// Recover effective weight gradually if it was decreased
		if e.effectiveWeight < e.backend.Weight {
			e.effectiveWeight++
		}

		if best == nil || e.currentWeight > best.currentWeight {
			best = e
		}
	}

	if best == nil {
		return nil, errNoHealthyBackends
	}

	best.currentWeight -= totalWeight
	return best.backend, nil
}

// AddBackend adds a backend with its configured weight to the weighted pool.
func (w *Weighted) AddBackend(b *Backend) {
	if b.Weight <= 0 {
		b.Weight = 1
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, &weightedEntry{
		backend:         b,
		effectiveWeight: b.Weight,
		currentWeight:   0,
	})
}

// RemoveBackend removes a backend by address from the weighted pool.
func (w *Weighted) RemoveBackend(address string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, e := range w.entries {
		if e.backend.Address == address {
			w.entries = append(w.entries[:i], w.entries[i+1:]...)
			return
		}
	}
}

// HealthCheck performs TCP health checks on all weighted backends.
func (w *Weighted) HealthCheck(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	backends := make([]*Backend, len(w.entries))
	for i, e := range w.entries {
		backends[i] = e.backend
	}
	performHealthChecks(ctx, backends)
}

// --- LeastConnections ---

// LeastConnections implements a least-connections load balancing strategy,
// selecting the healthy backend with the fewest active connections.
type LeastConnections struct {
	backends []*Backend
	mu       sync.RWMutex
}

// NewLeastConnections creates a new least-connections balancer.
func NewLeastConnections() *LeastConnections {
	return &LeastConnections{
		backends: make([]*Backend, 0),
	}
}

// Next returns the healthy backend with the fewest active connections.
// When there is a tie, the first backend found with the lowest count is returned.
func (lc *LeastConnections) Next() (*Backend, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if len(lc.backends) == 0 {
		return nil, errNoBackends
	}

	var best *Backend
	var minConns int64 = -1

	for _, b := range lc.backends {
		if !b.Healthy {
			continue
		}
		conns := atomic.LoadInt64(&b.ActiveConnections)
		if minConns < 0 || conns < minConns {
			minConns = conns
			best = b
		}
	}

	if best == nil {
		return nil, errNoHealthyBackends
	}

	atomic.AddInt64(&best.ActiveConnections, 1)
	return best, nil
}

// AddBackend adds a backend to the least-connections pool.
func (lc *LeastConnections) AddBackend(b *Backend) {
	if b.Weight == 0 {
		b.Weight = 1
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.backends = append(lc.backends, b)
}

// RemoveBackend removes a backend by address from the least-connections pool.
func (lc *LeastConnections) RemoveBackend(address string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	for i, b := range lc.backends {
		if b.Address == address {
			lc.backends = append(lc.backends[:i], lc.backends[i+1:]...)
			return
		}
	}
}

// HealthCheck performs TCP health checks on all backends.
func (lc *LeastConnections) HealthCheck(ctx context.Context) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	performHealthChecks(ctx, lc.backends)
}

// --- Pool ---

// Pool manages multiple named services, each with its own load balancer.
type Pool struct {
	balancers map[string]Balancer
	mu        sync.RWMutex
}

// NewPool creates a new service pool.
func NewPool() *Pool {
	return &Pool{
		balancers: make(map[string]Balancer),
	}
}

// AddService registers a new service with the specified balancing strategy.
// Supported strategies: "round-robin", "weighted", "least-connections".
func (p *Pool) AddService(name string, strategy string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.balancers[name]; exists {
		return fmt.Errorf("service %q already exists", name)
	}

	var balancer Balancer
	switch strategy {
	case "round-robin":
		balancer = NewRoundRobin()
	case "weighted":
		balancer = NewWeighted()
	case "least-connections":
		balancer = NewLeastConnections()
	default:
		return fmt.Errorf("unknown balancing strategy %q: supported strategies are round-robin, weighted, least-connections", strategy)
	}

	p.balancers[name] = balancer
	return nil
}

// GetBalancer returns the balancer for a named service.
func (p *Pool) GetBalancer(name string) (Balancer, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	balancer, exists := p.balancers[name]
	if !exists {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return balancer, nil
}

// RemoveService removes a service and its balancer from the pool.
func (p *Pool) RemoveService(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.balancers, name)
}

// HealthCheckAll runs health checks for all services in the pool.
func (p *Pool) HealthCheckAll(ctx context.Context) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, balancer := range p.balancers {
		select {
		case <-ctx.Done():
			return
		default:
			balancer.HealthCheck(ctx)
		}
	}
}

// --- Shared Utilities ---

// performHealthChecks runs TCP dial checks against all backends, updating
// their Healthy status and LastCheck timestamp.
func performHealthChecks(ctx context.Context, backends []*Backend) {
	const timeout = 2 * time.Second

	var wg sync.WaitGroup
	for _, b := range backends {
		wg.Add(1)
		go func(backend *Backend) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			dialer := net.Dialer{Timeout: timeout}
			conn, err := dialer.DialContext(ctx, "tcp", backend.Address)
			backend.LastCheck = time.Now()
			if err != nil {
				backend.Healthy = false
				return
			}
			conn.Close()
			backend.Healthy = true
		}(b)
	}
	wg.Wait()
}
