package loadbalancer

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinNoBackends(t *testing.T) {
	rr := NewRoundRobin()
	_, err := rr.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no backends available")
}

func TestRoundRobinCyclesHealthy(t *testing.T) {
	rr := NewRoundRobin()
	rr.AddBackend(&Backend{Address: "localhost:8001", Healthy: true})
	rr.AddBackend(&Backend{Address: "localhost:8002", Healthy: true})
	rr.AddBackend(&Backend{Address: "localhost:8003", Healthy: true})

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		b, err := rr.Next()
		require.NoError(t, err)
		seen[b.Address]++
	}
	// Each backend should have been hit 3 times in 9 calls.
	assert.Equal(t, 3, seen["localhost:8001"])
	assert.Equal(t, 3, seen["localhost:8002"])
	assert.Equal(t, 3, seen["localhost:8003"])
}

func TestRoundRobinSkipsUnhealthy(t *testing.T) {
	rr := NewRoundRobin()
	rr.AddBackend(&Backend{Address: "localhost:8001", Healthy: false})
	rr.AddBackend(&Backend{Address: "localhost:8002", Healthy: true})

	for i := 0; i < 5; i++ {
		b, err := rr.Next()
		require.NoError(t, err)
		assert.Equal(t, "localhost:8002", b.Address)
	}
}

func TestRoundRobinAllUnhealthy(t *testing.T) {
	rr := NewRoundRobin()
	rr.AddBackend(&Backend{Address: "localhost:8001", Healthy: false})
	rr.AddBackend(&Backend{Address: "localhost:8002", Healthy: false})

	_, err := rr.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy backends")
}

func TestRoundRobinRemoveBackend(t *testing.T) {
	rr := NewRoundRobin()
	rr.AddBackend(&Backend{Address: "localhost:8001", Healthy: true})
	rr.AddBackend(&Backend{Address: "localhost:8002", Healthy: true})

	rr.RemoveBackend("localhost:8001")

	b, err := rr.Next()
	require.NoError(t, err)
	assert.Equal(t, "localhost:8002", b.Address)
}

func TestRoundRobinDefaultWeight(t *testing.T) {
	rr := NewRoundRobin()
	b := &Backend{Address: "localhost:8001", Healthy: true, Weight: 0}
	rr.AddBackend(b)
	assert.Equal(t, 1, b.Weight)
}

func TestWeightedDistribution(t *testing.T) {
	w := NewWeighted()
	w.AddBackend(&Backend{Address: "a", Weight: 5, Healthy: true})
	w.AddBackend(&Backend{Address: "b", Weight: 1, Healthy: true})

	counts := map[string]int{}
	iterations := 600
	for i := 0; i < iterations; i++ {
		b, err := w.Next()
		require.NoError(t, err)
		counts[b.Address]++
	}
	// "a" has weight 5, "b" has weight 1 => "a" should get ~5x more
	assert.Greater(t, counts["a"], counts["b"]*3, "weighted backend 'a' should receive significantly more traffic")
}

func TestWeightedNoBackends(t *testing.T) {
	w := NewWeighted()
	_, err := w.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no backends available")
}

func TestWeightedAllUnhealthy(t *testing.T) {
	w := NewWeighted()
	w.AddBackend(&Backend{Address: "a", Weight: 1, Healthy: false})
	_, err := w.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy backends")
}

func TestWeightedRemoveBackend(t *testing.T) {
	w := NewWeighted()
	w.AddBackend(&Backend{Address: "a", Weight: 1, Healthy: true})
	w.AddBackend(&Backend{Address: "b", Weight: 1, Healthy: true})
	w.RemoveBackend("a")

	b, err := w.Next()
	require.NoError(t, err)
	assert.Equal(t, "b", b.Address)
}

func TestLeastConnectionsPicksLowest(t *testing.T) {
	lc := NewLeastConnections()
	b1 := &Backend{Address: "a", Healthy: true}
	b2 := &Backend{Address: "b", Healthy: true}
	atomic.StoreInt64(&b1.ActiveConnections, 10)
	atomic.StoreInt64(&b2.ActiveConnections, 2)

	lc.AddBackend(b1)
	lc.AddBackend(b2)

	b, err := lc.Next()
	require.NoError(t, err)
	assert.Equal(t, "b", b.Address)
}

func TestLeastConnectionsNoBackends(t *testing.T) {
	lc := NewLeastConnections()
	_, err := lc.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no backends available")
}

func TestLeastConnectionsAllUnhealthy(t *testing.T) {
	lc := NewLeastConnections()
	lc.AddBackend(&Backend{Address: "a", Healthy: false})
	_, err := lc.Next()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy backends")
}

func TestLeastConnectionsIncrementsConnections(t *testing.T) {
	lc := NewLeastConnections()
	b := &Backend{Address: "a", Healthy: true}
	lc.AddBackend(b)

	_, err := lc.Next()
	require.NoError(t, err)
	assert.Equal(t, int64(1), atomic.LoadInt64(&b.ActiveConnections))

	_, err = lc.Next()
	require.NoError(t, err)
	assert.Equal(t, int64(2), atomic.LoadInt64(&b.ActiveConnections))
}

func TestLeastConnectionsRemoveBackend(t *testing.T) {
	lc := NewLeastConnections()
	lc.AddBackend(&Backend{Address: "a", Healthy: true})
	lc.AddBackend(&Backend{Address: "b", Healthy: true})
	lc.RemoveBackend("a")

	b, err := lc.Next()
	require.NoError(t, err)
	assert.Equal(t, "b", b.Address)
}

func TestPoolAddAndGetService(t *testing.T) {
	p := NewPool()
	err := p.AddService("web", "round-robin")
	require.NoError(t, err)

	b, err := p.GetBalancer("web")
	require.NoError(t, err)
	assert.NotNil(t, b)
}

func TestPoolAddServiceDuplicate(t *testing.T) {
	p := NewPool()
	_ = p.AddService("web", "round-robin")
	err := p.AddService("web", "weighted")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestPoolUnknownStrategy(t *testing.T) {
	p := NewPool()
	err := p.AddService("web", "random")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown balancing strategy")
}

func TestPoolRemoveService(t *testing.T) {
	p := NewPool()
	_ = p.AddService("web", "round-robin")
	p.RemoveService("web")
	_, err := p.GetBalancer("web")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPoolStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
	}{
		{"round-robin", "round-robin"},
		{"weighted", "weighted"},
		{"least-connections", "least-connections"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPool()
			err := p.AddService("svc", tc.strategy)
			require.NoError(t, err)
			b, err := p.GetBalancer("svc")
			require.NoError(t, err)
			assert.NotNil(t, b)
		})
	}
}

func TestPoolHealthCheckAllCancelledContext(t *testing.T) {
	p := NewPool()
	_ = p.AddService("web", "round-robin")
	b, _ := p.GetBalancer("web")
	b.AddBackend(&Backend{Address: "localhost:99999", Healthy: true})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Should not panic or hang
	p.HealthCheckAll(ctx)
}

func TestBalancerInterface(t *testing.T) {
	// Verify all balancers implement the Balancer interface.
	var _ Balancer = NewRoundRobin()
	var _ Balancer = NewWeighted()
	var _ Balancer = NewLeastConnections()
}
