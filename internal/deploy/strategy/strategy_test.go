package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollingUpdateName(t *testing.T) {
	ru := NewRollingUpdate(3)
	assert.Equal(t, "rolling-update", ru.Name())
}

func TestRollingUpdateExecute(t *testing.T) {
	ru := NewRollingUpdate(2)
	ctx := context.Background()
	target := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v2",
		Replicas: 4,
		Port:     8080,
	}
	current := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v1",
		Replicas: 4,
		Port:     8080,
	}
	err := ru.Execute(ctx, current, target)
	assert.NoError(t, err)
}

func TestRollingUpdateNilTarget(t *testing.T) {
	ru := NewRollingUpdate(1)
	err := ru.Execute(context.Background(), nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target must not be nil")
}

func TestRollingUpdateCancelledContext(t *testing.T) {
	ru := NewRollingUpdate(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	target := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v2",
		Replicas: 3,
		Port:     8080,
	}
	err := ru.Execute(ctx, nil, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestRollingUpdateDefaultBatchSize(t *testing.T) {
	ru := NewRollingUpdate(0)
	assert.Equal(t, 1, ru.BatchSize)

	ru2 := NewRollingUpdate(-5)
	assert.Equal(t, 1, ru2.BatchSize)
}

func TestRollingUpdateRollback(t *testing.T) {
	ru := NewRollingUpdate(2)
	target := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v1",
		Replicas: 2,
		Port:     8080,
	}
	err := ru.Rollback(context.Background(), target)
	assert.NoError(t, err)
}

func TestBlueGreenExecute(t *testing.T) {
	bg := NewBlueGreen()
	bg.SwapDelay = 0 // skip delay in tests
	assert.Equal(t, "blue-green", bg.Name())

	ctx := context.Background()
	current := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v1",
		Replicas: 2,
		Port:     8080,
	}
	target := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v2",
		Replicas: 2,
		Port:     9090,
	}
	err := bg.Execute(ctx, current, target)
	assert.NoError(t, err)
}

func TestBlueGreenNilTarget(t *testing.T) {
	bg := NewBlueGreen()
	err := bg.Execute(context.Background(), nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target must not be nil")
}

func TestCanaryExecute(t *testing.T) {
	c := NewCanary(10)
	c.StepInterval = 1 * time.Millisecond
	c.MonitorDuration = 1 * time.Millisecond
	c.StepIncrement = 50
	assert.Equal(t, "canary", c.Name())

	ctx := context.Background()
	current := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v1",
		Replicas: 4,
		Port:     8080,
	}
	target := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v2",
		Replicas: 4,
		Port:     9090,
	}
	err := c.Execute(ctx, current, target)
	assert.NoError(t, err)
}

func TestCanaryWeightClamping(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"below minimum", -5, 1},
		{"at minimum", 1, 1},
		{"normal", 50, 50},
		{"at maximum", 100, 100},
		{"above maximum", 200, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCanary(tc.input)
			assert.Equal(t, tc.expected, c.Weight)
		})
	}
}

func TestCanaryRollback(t *testing.T) {
	c := NewCanary(10)
	target := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v1",
		Replicas: 2,
		Port:     8080,
	}
	err := c.Rollback(context.Background(), target)
	assert.NoError(t, err)
}

func TestCanaryNilTarget(t *testing.T) {
	c := NewCanary(10)
	err := c.Execute(context.Background(), nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target must not be nil")
}

func TestComputeCanaryReplicas(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		weight   int
		expected int
	}{
		{"10% of 10", 10, 10, 1},
		{"50% of 10", 10, 50, 5},
		{"100% of 10", 10, 100, 10},
		{"10% of 1 rounds up to 1", 1, 10, 1},
		{"zero total defaults to 1", 0, 50, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := computeCanaryReplicas(tc.total, tc.weight)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStrategyInterface(t *testing.T) {
	// Verify all strategies implement the Strategy interface.
	var _ Strategy = NewRollingUpdate(1)
	var _ Strategy = NewBlueGreen()
	var _ Strategy = NewCanary(10)
}

func TestRollingUpdateWithHealthCheck(t *testing.T) {
	ru := NewRollingUpdate(1)
	target := &DeployTarget{
		Name:     "myapp",
		Image:    "myapp:v2",
		Replicas: 2,
		Port:     8080,
		HealthCheck: &HealthCheck{
			Path:     "/health",
			Interval: 1 * time.Millisecond,
			Timeout:  1 * time.Millisecond,
			Retries:  1,
		},
	}
	err := ru.Execute(context.Background(), nil, target)
	require.NoError(t, err)
}
