package alert

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChannel records alerts sent to it.
type mockChannel struct {
	mu     sync.Mutex
	name   string
	alerts []*Alert
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Send(_ context.Context, alert *Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, alert)
	return nil
}

func (m *mockChannel) received() []*Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*Alert, len(m.alerts))
	copy(cp, m.alerts)
	return cp
}

func TestAddRuleAndGetRule(t *testing.T) {
	eng := NewEngine()

	rule := &Rule{
		ID:        "rule-1",
		Name:      "High CPU",
		Condition: "cpu_usage",
		Threshold: 90.0,
		Duration:  0,
		Severity:  SeverityCritical,
		Channels:  []string{"slack"},
		Enabled:   true,
	}

	eng.AddRule(rule)

	got := eng.GetRule("rule-1")
	require.NotNil(t, got)
	assert.Equal(t, "High CPU", got.Name)
	assert.Equal(t, 90.0, got.Threshold)

	// Auto-generate ID when empty
	rule2 := &Rule{
		Name:      "Low Disk",
		Condition: "disk_free",
		Threshold: 10.0,
		Enabled:   true,
	}
	eng.AddRule(rule2)
	assert.NotEmpty(t, rule2.ID)
	assert.NotNil(t, eng.GetRule(rule2.ID))

	// Nonexistent
	assert.Nil(t, eng.GetRule("nonexistent"))
}

func TestRemoveRule(t *testing.T) {
	eng := NewEngine()

	rule := &Rule{
		ID:        "rule-1",
		Name:      "Test",
		Condition: "cpu",
		Threshold: 50.0,
		Duration:  0,
		Severity:  SeverityWarning,
		Channels:  []string{},
		Enabled:   true,
	}
	eng.AddRule(rule)

	// Trigger an alert
	eng.Evaluate("cpu", 60.0)
	assert.Len(t, eng.ActiveAlerts(), 1)

	// Remove the rule — active alert should be resolved
	eng.RemoveRule("rule-1")
	assert.Nil(t, eng.GetRule("rule-1"))
	assert.Empty(t, eng.ActiveAlerts())

	// Remove nonexistent is a no-op
	eng.RemoveRule("nonexistent")
}

func TestEvaluateFiresAndResolves(t *testing.T) {
	eng := NewEngine()
	ch := &mockChannel{name: "test-ch"}
	eng.AddChannel("test-ch", ch)

	rule := &Rule{
		ID:        "rule-1",
		Name:      "High Memory",
		Condition: "mem_usage",
		Threshold: 80.0,
		Duration:  0, // fires immediately
		Severity:  SeverityWarning,
		Labels:    map[string]string{"service": "api"},
		Channels:  []string{"test-ch"},
		Enabled:   true,
	}
	eng.AddRule(rule)

	// Below threshold — no alert
	eng.Evaluate("mem_usage", 70.0)
	assert.Empty(t, eng.ActiveAlerts())

	// Above threshold — should fire
	eng.Evaluate("mem_usage", 85.0)
	alerts := eng.ActiveAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, AlertFiring, alerts[0].Status)
	assert.Equal(t, "rule-1", alerts[0].RuleID)
	assert.Equal(t, "api", alerts[0].Labels["service"])

	// Evaluating again above threshold should NOT create a second alert
	eng.Evaluate("mem_usage", 90.0)
	assert.Len(t, eng.ActiveAlerts(), 1)

	// Drop below threshold — should resolve
	eng.Evaluate("mem_usage", 50.0)
	assert.Empty(t, eng.ActiveAlerts())

	// Give async dispatch a moment
	time.Sleep(50 * time.Millisecond)

	// Channel should have received firing + resolved (order may vary due to async dispatch)
	received := ch.received()
	require.Len(t, received, 2)
	statuses := []AlertStatus{received[0].Status, received[1].Status}
	assert.Contains(t, statuses, AlertFiring)
	assert.Contains(t, statuses, AlertResolved)
}

func TestEvaluateWithDuration(t *testing.T) {
	eng := NewEngine()

	rule := &Rule{
		ID:        "rule-dur",
		Name:      "Sustained CPU",
		Condition: "cpu",
		Threshold: 80.0,
		Duration:  100 * time.Millisecond,
		Severity:  SeverityCritical,
		Channels:  []string{},
		Enabled:   true,
	}
	eng.AddRule(rule)

	// First evaluation above threshold — starts pending but no alert yet
	eng.Evaluate("cpu", 90.0)
	assert.Empty(t, eng.ActiveAlerts())

	// Still pending, not enough time
	eng.Evaluate("cpu", 95.0)
	assert.Empty(t, eng.ActiveAlerts())

	// Wait for duration to pass
	time.Sleep(150 * time.Millisecond)

	// Now it should fire
	eng.Evaluate("cpu", 92.0)
	assert.Len(t, eng.ActiveAlerts(), 1)
}

func TestEvaluateDisabledRule(t *testing.T) {
	eng := NewEngine()

	rule := &Rule{
		ID:        "disabled",
		Name:      "Disabled Rule",
		Condition: "metric",
		Threshold: 10.0,
		Duration:  0,
		Enabled:   false,
	}
	eng.AddRule(rule)

	eng.Evaluate("metric", 100.0)
	assert.Empty(t, eng.ActiveAlerts())
}

func TestEvaluateConditionMismatch(t *testing.T) {
	eng := NewEngine()

	rule := &Rule{
		ID:        "cpu-rule",
		Name:      "CPU Rule",
		Condition: "cpu",
		Threshold: 50.0,
		Duration:  0,
		Enabled:   true,
	}
	eng.AddRule(rule)

	// Evaluate a different metric — should not fire
	eng.Evaluate("memory", 100.0)
	assert.Empty(t, eng.ActiveAlerts())
}

func TestStartAndStop(t *testing.T) {
	eng := NewEngine()

	rule := &Rule{
		ID:        "auto",
		Name:      "Auto Check",
		Condition: "latency",
		Threshold: 100.0,
		Duration:  0,
		Severity:  SeverityInfo,
		Channels:  []string{},
		Enabled:   true,
	}
	eng.AddRule(rule)

	callCount := 0
	var mu sync.Mutex

	metricsFunc := func() map[string]float64 {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		return map[string]float64{"latency": 200.0}
	}

	ctx := context.Background()
	eng.Start(ctx, 50*time.Millisecond, metricsFunc)

	time.Sleep(200 * time.Millisecond)
	eng.Stop()

	mu.Lock()
	count := callCount
	mu.Unlock()
	assert.Greater(t, count, 0, "metricsFunc should have been called at least once")
	assert.Len(t, eng.ActiveAlerts(), 1)
}

func TestChannelNames(t *testing.T) {
	assert.Equal(t, "slack", (&SlackChannel{}).Name())
	assert.Equal(t, "discord", (&DiscordChannel{}).Name())
	assert.Equal(t, "email", (&EmailChannel{}).Name())
	assert.Equal(t, "webhook", (&WebhookChannel{}).Name())
}
