package alert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	require.NotNil(t, e)
	assert.NotNil(t, e.rules)
	assert.NotNil(t, e.activeAlerts)
	assert.NotNil(t, e.channels)
	assert.NotNil(t, e.pending)
	assert.Empty(t, e.rules)
	assert.Empty(t, e.activeAlerts)
}

func TestAddRule(t *testing.T) {
	e := NewEngine()

	rule := &Rule{
		ID:        "rule-1",
		Name:      "High CPU",
		Condition: "cpu_usage",
		Threshold: 0.9,
		Duration:  5 * time.Minute,
		Severity:  SeverityCritical,
		Channels:  []string{"slack"},
		Enabled:   true,
	}

	e.AddRule(rule)

	got := e.GetRule("rule-1")
	require.NotNil(t, got)
	assert.Equal(t, "rule-1", got.ID)
	assert.Equal(t, "High CPU", got.Name)
	assert.Equal(t, "cpu_usage", got.Condition)
	assert.Equal(t, 0.9, got.Threshold)
	assert.Equal(t, SeverityCritical, got.Severity)
	assert.True(t, got.Enabled)
}

func TestAddRule_GeneratesID(t *testing.T) {
	e := NewEngine()

	rule := &Rule{
		Name:      "No ID Rule",
		Condition: "memory",
		Threshold: 80,
		Severity:  SeverityWarning,
		Enabled:   true,
	}

	e.AddRule(rule)
	assert.NotEmpty(t, rule.ID, "ID should be auto-generated when empty")
	got := e.GetRule(rule.ID)
	require.NotNil(t, got)
	assert.Equal(t, "No ID Rule", got.Name)
}

func TestRemoveRule(t *testing.T) {
	e := NewEngine()

	rule := &Rule{
		ID:        "rule-to-remove",
		Name:      "Temp Rule",
		Condition: "disk_usage",
		Threshold: 0.95,
		Severity:  SeverityWarning,
		Enabled:   true,
	}

	e.AddRule(rule)
	require.NotNil(t, e.GetRule("rule-to-remove"))

	e.RemoveRule("rule-to-remove")
	assert.Nil(t, e.GetRule("rule-to-remove"))
}

func TestRemoveRule_ResolvesActiveAlert(t *testing.T) {
	e := NewEngine()

	rule := &Rule{
		ID:        "rule-active",
		Name:      "Active Rule",
		Condition: "cpu_usage",
		Threshold: 0.5,
		Duration:  0, // fires immediately
		Severity:  SeverityCritical,
		Channels:  []string{},
		Enabled:   true,
	}

	e.AddRule(rule)

	// Trigger an alert by evaluating a value above threshold
	e.Evaluate("cpu_usage", 0.8)
	assert.Len(t, e.ActiveAlerts(), 1)

	// Removing the rule should resolve and clean up the active alert
	e.RemoveRule("rule-active")
	assert.Empty(t, e.ActiveAlerts())
	assert.Nil(t, e.GetRule("rule-active"))
}

func TestListRules(t *testing.T) {
	e := NewEngine()

	rules := []*Rule{
		{ID: "r1", Name: "Rule 1", Condition: "cpu", Threshold: 0.5, Severity: SeverityInfo, Enabled: true},
		{ID: "r2", Name: "Rule 2", Condition: "mem", Threshold: 0.8, Severity: SeverityWarning, Enabled: true},
		{ID: "r3", Name: "Rule 3", Condition: "disk", Threshold: 0.9, Severity: SeverityCritical, Enabled: true},
	}

	for _, r := range rules {
		e.AddRule(r)
	}

	// Verify all rules are retrievable
	for _, r := range rules {
		got := e.GetRule(r.ID)
		require.NotNil(t, got, "rule %s should exist", r.ID)
		assert.Equal(t, r.Name, got.Name)
	}

	// Verify the count by checking each rule exists
	count := 0
	for _, r := range rules {
		if e.GetRule(r.ID) != nil {
			count++
		}
	}
	assert.Equal(t, 3, count)
}

func TestAlertChannelTypes(t *testing.T) {
	// Validate Severity constants
	assert.Equal(t, Severity("info"), SeverityInfo)
	assert.Equal(t, Severity("warning"), SeverityWarning)
	assert.Equal(t, Severity("critical"), SeverityCritical)

	// Validate AlertStatus constants
	assert.Equal(t, AlertStatus("firing"), AlertFiring)
	assert.Equal(t, AlertStatus("resolved"), AlertResolved)

	// Validate channel Name() methods return expected type names
	slack := &SlackChannel{WebhookURL: "https://hooks.slack.com/test", Channel: "#alerts"}
	assert.Equal(t, "slack", slack.Name())

	discord := &DiscordChannel{WebhookURL: "https://discord.com/api/webhooks/test"}
	assert.Equal(t, "discord", discord.Name())

	email := &EmailChannel{SMTPHost: "smtp.example.com:587", From: "alerts@example.com", To: []string{"team@example.com"}}
	assert.Equal(t, "email", email.Name())

	webhook := &WebhookChannel{URL: "https://example.com/webhook", Headers: map[string]string{"X-Token": "abc"}}
	assert.Equal(t, "webhook", webhook.Name())
}
