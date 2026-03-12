package strategy

import (
	"context"
	"fmt"
	"time"
)

// HealthCheck defines how to verify a deployment target is healthy.
type HealthCheck struct {
	Path     string
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

// DeployTarget represents a single version of a service that can be deployed
// or rolled back.
type DeployTarget struct {
	Name        string
	Image       string
	Replicas    int
	Port        int
	Env         map[string]string
	HealthCheck *HealthCheck
}

// Strategy is the contract every deployment strategy must satisfy.
type Strategy interface {
	// Execute transitions traffic from current to target.  current may be nil
	// for a first deploy.
	Execute(ctx context.Context, current, target *DeployTarget) error

	// Rollback reverts to the given target (typically the previous version).
	Rollback(ctx context.Context, target *DeployTarget) error

	// Name returns a human-readable identifier for this strategy.
	Name() string
}

// ---------------------------------------------------------------------------
// RollingUpdate
// ---------------------------------------------------------------------------

// RollingUpdate replaces containers one batch at a time, running a health
// check after each batch before proceeding.
type RollingUpdate struct {
	BatchSize int
}

// NewRollingUpdate creates a RollingUpdate strategy that replaces batchSize
// containers at a time.  If batchSize <= 0 it defaults to 1.
func NewRollingUpdate(batchSize int) *RollingUpdate {
	if batchSize <= 0 {
		batchSize = 1
	}
	return &RollingUpdate{BatchSize: batchSize}
}

func (r *RollingUpdate) Name() string { return "rolling-update" }

// Execute performs a rolling update from current to target.
func (r *RollingUpdate) Execute(ctx context.Context, current, target *DeployTarget) error {
	if target == nil {
		return fmt.Errorf("target must not be nil")
	}

	totalReplicas := target.Replicas
	if totalReplicas <= 0 {
		totalReplicas = 1
	}

	replaced := 0
	for replaced < totalReplicas {
		select {
		case <-ctx.Done():
			return fmt.Errorf("rolling update cancelled: %w", ctx.Err())
		default:
		}

		batchEnd := replaced + r.BatchSize
		if batchEnd > totalReplicas {
			batchEnd = totalReplicas
		}

		// Stop old replicas in this batch.
		if current != nil {
			for i := replaced; i < batchEnd; i++ {
				if err := stopReplica(ctx, current, i); err != nil {
					return fmt.Errorf("stop old replica %d: %w", i, err)
				}
			}
		}

		// Start new replicas in this batch.
		for i := replaced; i < batchEnd; i++ {
			if err := startReplica(ctx, target, i); err != nil {
				return fmt.Errorf("start new replica %d: %w", i, err)
			}
		}

		// Health check after each batch.
		if target.HealthCheck != nil {
			if err := checkHealth(ctx, target, replaced, batchEnd); err != nil {
				return fmt.Errorf("health check failed for batch [%d..%d): %w", replaced, batchEnd, err)
			}
		}

		replaced = batchEnd
	}

	return nil
}

// Rollback performs a rolling update back to the given target.
func (r *RollingUpdate) Rollback(ctx context.Context, target *DeployTarget) error {
	if target == nil {
		return fmt.Errorf("rollback target must not be nil")
	}
	// A rollback is simply a rolling update to the old version with no
	// "current" to tear down first (the caller has already marked the
	// failed deployment).
	return r.Execute(ctx, nil, target)
}

// ---------------------------------------------------------------------------
// BlueGreen
// ---------------------------------------------------------------------------

// BlueGreen deploys the new version alongside the old one, switches traffic
// atomically, then tears down the old version.
type BlueGreen struct {
	SwapDelay time.Duration // optional delay before tearing down old
}

// NewBlueGreen creates a BlueGreen strategy with sensible defaults.
func NewBlueGreen() *BlueGreen {
	return &BlueGreen{SwapDelay: 5 * time.Second}
}

func (bg *BlueGreen) Name() string { return "blue-green" }

// Execute brings up the green environment, verifies health, switches traffic,
// and tears down the blue (old) environment.
func (bg *BlueGreen) Execute(ctx context.Context, current, target *DeployTarget) error {
	if target == nil {
		return fmt.Errorf("target must not be nil")
	}

	replicas := target.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	// 1. Deploy all green replicas.
	for i := 0; i < replicas; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("blue-green cancelled during green deploy: %w", ctx.Err())
		default:
		}
		if err := startReplica(ctx, target, i); err != nil {
			return fmt.Errorf("start green replica %d: %w", i, err)
		}
	}

	// 2. Verify green is healthy.
	if target.HealthCheck != nil {
		if err := checkHealth(ctx, target, 0, replicas); err != nil {
			// Tear down the unhealthy green set.
			for i := 0; i < replicas; i++ {
				_ = stopReplica(ctx, target, i)
			}
			return fmt.Errorf("green health check failed, rolled back: %w", err)
		}
	}

	// 3. Switch traffic (atomic pointer swap in a real load balancer).
	if err := switchTraffic(ctx, current, target); err != nil {
		return fmt.Errorf("traffic switch failed: %w", err)
	}

	// 4. Optionally wait before tearing down old (drain connections).
	if bg.SwapDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(bg.SwapDelay):
		}
	}

	// 5. Tear down old (blue) replicas.
	if current != nil {
		oldReplicas := current.Replicas
		if oldReplicas <= 0 {
			oldReplicas = 1
		}
		for i := 0; i < oldReplicas; i++ {
			if err := stopReplica(ctx, current, i); err != nil {
				return fmt.Errorf("stop blue replica %d: %w", i, err)
			}
		}
	}

	return nil
}

// Rollback switches traffic back to the previous target and removes the
// failed green deployment.
func (bg *BlueGreen) Rollback(ctx context.Context, target *DeployTarget) error {
	if target == nil {
		return fmt.Errorf("rollback target must not be nil")
	}

	replicas := target.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	// Re-deploy old version.
	for i := 0; i < replicas; i++ {
		if err := startReplica(ctx, target, i); err != nil {
			return fmt.Errorf("rollback start replica %d: %w", i, err)
		}
	}

	if err := switchTraffic(ctx, nil, target); err != nil {
		return fmt.Errorf("rollback traffic switch: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Canary
// ---------------------------------------------------------------------------

// Canary deploys a small percentage of traffic to the new version, monitors
// health, and then completes the rollout.
type Canary struct {
	Weight          int           // initial traffic percentage (1-100)
	StepInterval    time.Duration // time between weight increases
	StepIncrement   int           // percentage increase per step
	MonitorDuration time.Duration // how long to watch the canary at each step
}

// NewCanary creates a Canary strategy starting at initialWeight percent of
// traffic.  If initialWeight is out of range it is clamped.
func NewCanary(initialWeight int) *Canary {
	if initialWeight < 1 {
		initialWeight = 1
	}
	if initialWeight > 100 {
		initialWeight = 100
	}
	return &Canary{
		Weight:          initialWeight,
		StepInterval:    30 * time.Second,
		StepIncrement:   10,
		MonitorDuration: 15 * time.Second,
	}
}

func (c *Canary) Name() string { return "canary" }

// Execute gradually shifts traffic to the target version.
func (c *Canary) Execute(ctx context.Context, current, target *DeployTarget) error {
	if target == nil {
		return fmt.Errorf("target must not be nil")
	}

	canaryReplicas := computeCanaryReplicas(target.Replicas, c.Weight)
	if canaryReplicas < 1 {
		canaryReplicas = 1
	}

	// 1. Deploy canary replicas.
	for i := 0; i < canaryReplicas; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("canary deploy cancelled: %w", ctx.Err())
		default:
		}
		if err := startReplica(ctx, target, i); err != nil {
			return fmt.Errorf("start canary replica %d: %w", i, err)
		}
	}

	// 2. Set initial traffic weight.
	if err := setTrafficWeight(ctx, target, c.Weight); err != nil {
		return fmt.Errorf("set canary weight %d: %w", c.Weight, err)
	}

	// 3. Monitor canary at initial weight.
	if target.HealthCheck != nil {
		if err := monitorCanary(ctx, target, c.MonitorDuration); err != nil {
			// Abort: remove canary replicas.
			for i := 0; i < canaryReplicas; i++ {
				_ = stopReplica(ctx, target, i)
			}
			_ = setTrafficWeight(ctx, target, 0)
			return fmt.Errorf("canary health failed at weight %d%%: %w", c.Weight, err)
		}
	}

	// 4. Gradually increase weight to 100%.
	currentWeight := c.Weight
	for currentWeight < 100 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("canary promotion cancelled: %w", ctx.Err())
		case <-time.After(c.StepInterval):
		}

		currentWeight += c.StepIncrement
		if currentWeight > 100 {
			currentWeight = 100
		}

		if err := setTrafficWeight(ctx, target, currentWeight); err != nil {
			return fmt.Errorf("set weight %d: %w", currentWeight, err)
		}

		// Add replicas to match the weight.
		needed := computeCanaryReplicas(target.Replicas, currentWeight)
		for i := canaryReplicas; i < needed; i++ {
			if err := startReplica(ctx, target, i); err != nil {
				return fmt.Errorf("scale canary replica %d: %w", i, err)
			}
		}
		canaryReplicas = needed

		// Health-check at new weight.
		if target.HealthCheck != nil {
			if err := monitorCanary(ctx, target, c.MonitorDuration); err != nil {
				return fmt.Errorf("canary health failed at weight %d%%: %w", currentWeight, err)
			}
		}
	}

	// 5. Tear down old replicas.
	if current != nil {
		oldReplicas := current.Replicas
		if oldReplicas <= 0 {
			oldReplicas = 1
		}
		for i := 0; i < oldReplicas; i++ {
			_ = stopReplica(ctx, current, i)
		}
	}

	return nil
}

// Rollback removes canary replicas and restores full traffic to the previous
// version.
func (c *Canary) Rollback(ctx context.Context, target *DeployTarget) error {
	if target == nil {
		return fmt.Errorf("rollback target must not be nil")
	}

	if err := setTrafficWeight(ctx, target, 100); err != nil {
		return fmt.Errorf("rollback weight reset: %w", err)
	}

	replicas := target.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	for i := 0; i < replicas; i++ {
		if err := startReplica(ctx, target, i); err != nil {
			return fmt.Errorf("rollback start replica %d: %w", i, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// shared helpers (in production these call the real infra layer)
// ---------------------------------------------------------------------------

func startReplica(ctx context.Context, t *DeployTarget, index int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_ = fmt.Sprintf("starting %s replica %d with image %s on port %d", t.Name, index, t.Image, t.Port+index)
	return nil
}

func stopReplica(ctx context.Context, t *DeployTarget, index int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func checkHealth(ctx context.Context, t *DeployTarget, from, to int) error {
	if t.HealthCheck == nil {
		return nil
	}
	retries := t.HealthCheck.Retries
	if retries <= 0 {
		retries = 3
	}
	for i := from; i < to; i++ {
		healthy := false
		for attempt := 0; attempt < retries; attempt++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(t.HealthCheck.Interval):
			}
			// Would HTTP GET http://localhost:<port+i><path> here.
			healthy = true
			break
		}
		if !healthy {
			return fmt.Errorf("replica %d unhealthy after %d attempts", i, retries)
		}
	}
	return nil
}

func switchTraffic(_ context.Context, _, target *DeployTarget) error {
	if target == nil {
		return fmt.Errorf("cannot switch traffic: target is nil")
	}
	// In production: update load balancer / service mesh route.
	return nil
}

func setTrafficWeight(_ context.Context, target *DeployTarget, weight int) error {
	if weight < 0 || weight > 100 {
		return fmt.Errorf("weight %d out of range [0,100]", weight)
	}
	_ = fmt.Sprintf("setting traffic weight for %s to %d%%", target.Name, weight)
	return nil
}

func monitorCanary(ctx context.Context, target *DeployTarget, duration time.Duration) error {
	deadline := time.Now().Add(duration)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			if t.After(deadline) {
				return nil // monitoring window passed, canary is healthy
			}
			// Would check error rate, latency, etc.
		}
	}
}

func computeCanaryReplicas(total, weightPct int) int {
	if total <= 0 {
		total = 1
	}
	n := (total * weightPct) / 100
	if n < 1 {
		n = 1
	}
	return n
}
