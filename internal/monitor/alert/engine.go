// Package alert implements a rules-based alert engine with pluggable
// notification channels (Slack, Discord, Email, generic Webhook).
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Severity indicates how critical an alert is.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// AlertStatus indicates whether an alert is currently firing or resolved.
type AlertStatus string

const (
	AlertFiring   AlertStatus = "firing"
	AlertResolved AlertStatus = "resolved"
)

// Rule defines a condition under which an alert should fire.
type Rule struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Condition string            `json:"condition"` // metric name to evaluate
	Threshold float64           `json:"threshold"`
	Duration  time.Duration     `json:"duration"` // how long the condition must hold
	Severity  Severity          `json:"severity"`
	Labels    map[string]string `json:"labels,omitempty"`
	Channels  []string          `json:"channels"`
	Enabled   bool              `json:"enabled"`
}

// Alert represents an active or resolved alert instance.
type Alert struct {
	ID          string            `json:"id"`
	RuleID      string            `json:"rule_id"`
	Status      AlertStatus       `json:"status"`
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      time.Time         `json:"ends_at,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Channel is the interface each notification backend must implement.
type Channel interface {
	Send(ctx context.Context, alert *Alert) error
	Name() string
}

// ---------- SlackChannel ----------

// SlackChannel sends alert notifications to a Slack incoming webhook.
type SlackChannel struct {
	WebhookURL string
	Channel    string
}

func (s *SlackChannel) Name() string { return "slack" }

// Send posts a formatted message to the Slack webhook.
func (s *SlackChannel) Send(ctx context.Context, alert *Alert) error {
	color := "#36a64f"
	if alert.Status == AlertFiring {
		color = "#ff0000"
	}

	payload := map[string]interface{}{
		"channel": s.Channel,
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  fmt.Sprintf("[%s] Alert %s", strings.ToUpper(string(alert.Status)), alert.RuleID),
				"text":   formatAlertText(alert),
				"ts":     alert.StartsAt.Unix(),
				"fields": labelsToSlackFields(alert.Labels),
			},
		},
	}

	return postJSON(ctx, s.WebhookURL, payload)
}

func labelsToSlackFields(labels map[string]string) []map[string]interface{} {
	fields := make([]map[string]interface{}, 0, len(labels))
	for k, v := range labels {
		fields = append(fields, map[string]interface{}{
			"title": k,
			"value": v,
			"short": true,
		})
	}
	return fields
}

// ---------- DiscordChannel ----------

// DiscordChannel sends alert notifications to a Discord webhook.
type DiscordChannel struct {
	WebhookURL string
}

func (d *DiscordChannel) Name() string { return "discord" }

// Send posts an embed message to the Discord webhook.
func (d *DiscordChannel) Send(ctx context.Context, alert *Alert) error {
	colorCode := 0x00ff00 // green for resolved
	if alert.Status == AlertFiring {
		colorCode = 0xff0000 // red for firing
	}

	embedFields := make([]map[string]interface{}, 0, len(alert.Labels))
	for k, v := range alert.Labels {
		embedFields = append(embedFields, map[string]interface{}{
			"name":   k,
			"value":  v,
			"inline": true,
		})
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("[%s] Alert %s", strings.ToUpper(string(alert.Status)), alert.RuleID),
				"description": formatAlertText(alert),
				"color":       colorCode,
				"timestamp":   alert.StartsAt.Format(time.RFC3339),
				"fields":      embedFields,
			},
		},
	}

	return postJSON(ctx, d.WebhookURL, payload)
}

// ---------- EmailChannel ----------

// EmailChannel sends alert notifications via SMTP.
type EmailChannel struct {
	SMTPHost string
	From     string
	To       []string
}

func (e *EmailChannel) Name() string { return "email" }

// Send delivers an alert email through the configured SMTP server.
func (e *EmailChannel) Send(_ context.Context, alert *Alert) error {
	subject := fmt.Sprintf("[NexusOps %s] %s - %s",
		strings.ToUpper(string(alert.Status)), alert.RuleID, alert.ID)

	body := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n\r\nLabels:\r\n",
		subject, e.From, strings.Join(e.To, ","), formatAlertText(alert))

	for k, v := range alert.Labels {
		body += fmt.Sprintf("  %s = %s\r\n", k, v)
	}
	for k, v := range alert.Annotations {
		body += fmt.Sprintf("  %s = %s\r\n", k, v)
	}

	return smtp.SendMail(e.SMTPHost, nil, e.From, e.To, []byte(body))
}

// ---------- WebhookChannel ----------

// WebhookChannel sends alert notifications as JSON POST requests to an
// arbitrary URL.
type WebhookChannel struct {
	URL     string
	Headers map[string]string
}

func (w *WebhookChannel) Name() string { return "webhook" }

// Send posts the alert as JSON to the configured URL.
func (w *WebhookChannel) Send(ctx context.Context, alert *Alert) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// ---------- Engine ----------

// pendingViolation tracks how long a rule condition has been continuously true.
type pendingViolation struct {
	firstSeen time.Time
	lastValue float64
}

// Engine evaluates alert rules against metric values and dispatches
// notifications through registered channels.
type Engine struct {
	mu           sync.RWMutex
	rules        map[string]*Rule
	activeAlerts map[string]*Alert // keyed by rule ID
	channels     map[string]Channel
	pending      map[string]*pendingViolation // keyed by rule ID

	cancel context.CancelFunc
	done   chan struct{}
}

// NewEngine creates a new alert Engine.
func NewEngine() *Engine {
	return &Engine{
		rules:        make(map[string]*Rule),
		activeAlerts: make(map[string]*Alert),
		channels:     make(map[string]Channel),
		pending:      make(map[string]*pendingViolation),
	}
}

// AddRule registers or updates an alerting rule.
func (e *Engine) AddRule(rule *Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	e.rules[rule.ID] = rule
}

// RemoveRule deletes a rule and resolves any active alert associated with it.
func (e *Engine) RemoveRule(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.rules, id)
	if a, ok := e.activeAlerts[id]; ok {
		a.Status = AlertResolved
		a.EndsAt = time.Now()
		delete(e.activeAlerts, id)
	}
	delete(e.pending, id)
}

// AddChannel registers a notification channel under the given name.
func (e *Engine) AddChannel(name string, ch Channel) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.channels[name] = ch
}

// Evaluate checks a metric value against all enabled rules whose Condition
// matches the metricName. It fires or resolves alerts accordingly.
func (e *Engine) Evaluate(metricName string, value float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Condition != metricName {
			continue
		}

		violated := value > rule.Threshold

		if violated {
			pv, exists := e.pending[rule.ID]
			if !exists {
				e.pending[rule.ID] = &pendingViolation{
					firstSeen: now,
					lastValue: value,
				}
				pv = e.pending[rule.ID]
			} else {
				pv.lastValue = value
			}

			// Only fire if the violation has persisted for the rule's Duration.
			if now.Sub(pv.firstSeen) >= rule.Duration {
				if _, alreadyFiring := e.activeAlerts[rule.ID]; !alreadyFiring {
					alert := &Alert{
						ID:     uuid.New().String(),
						RuleID: rule.ID,
						Status: AlertFiring,
						StartsAt: pv.firstSeen,
						Labels: mergeLabels(rule.Labels, map[string]string{
							"metric":   metricName,
							"severity": string(rule.Severity),
						}),
						Annotations: map[string]string{
							"value":     fmt.Sprintf("%.4f", value),
							"threshold": fmt.Sprintf("%.4f", rule.Threshold),
							"rule":      rule.Name,
						},
					}
					e.activeAlerts[rule.ID] = alert
					e.dispatchAlert(alert, rule.Channels)
				}
			}
		} else {
			// Condition no longer violated — resolve if firing.
			delete(e.pending, rule.ID)
			if existing, ok := e.activeAlerts[rule.ID]; ok {
				existing.Status = AlertResolved
				existing.EndsAt = now
				e.dispatchAlert(existing, rule.Channels)
				delete(e.activeAlerts, rule.ID)
			}
		}
	}
}

// dispatchAlert sends the alert to all named channels. It is called with the
// engine lock held; the actual sends happen asynchronously.
func (e *Engine) dispatchAlert(alert *Alert, channelNames []string) {
	for _, name := range channelNames {
		ch, ok := e.channels[name]
		if !ok {
			continue
		}
		// Copy for goroutine safety.
		a := *alert
		c := ch
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = c.Send(ctx, &a)
		}()
	}
}

// Start begins periodic evaluation. The provided metricsFunc is called every
// checkInterval to obtain the current metric values, which are then fed into
// Evaluate.
func (e *Engine) Start(ctx context.Context, checkInterval time.Duration, metricsFunc func() map[string]float64) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.done = make(chan struct{})

	go func() {
		defer close(e.done)
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics := metricsFunc()
				for name, val := range metrics {
					e.Evaluate(name, val)
				}
			}
		}
	}()
}

// Stop cancels the background evaluation loop.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.done != nil {
		<-e.done
	}
}

// ActiveAlerts returns a snapshot of all currently firing alerts.
func (e *Engine) ActiveAlerts() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	alerts := make([]*Alert, 0, len(e.activeAlerts))
	for _, a := range e.activeAlerts {
		alerts = append(alerts, a)
	}
	return alerts
}

// GetRule returns the rule with the given ID, or nil if not found.
func (e *Engine) GetRule(id string) *Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rules[id]
}

// ---------- helpers ----------

func formatAlertText(a *Alert) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Alert ID: %s\n", a.ID))
	sb.WriteString(fmt.Sprintf("Rule: %s\n", a.RuleID))
	sb.WriteString(fmt.Sprintf("Status: %s\n", a.Status))
	sb.WriteString(fmt.Sprintf("Started: %s\n", a.StartsAt.Format(time.RFC3339)))
	if !a.EndsAt.IsZero() {
		sb.WriteString(fmt.Sprintf("Ended: %s\n", a.EndsAt.Format(time.RFC3339)))
	}
	for k, v := range a.Annotations {
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	return sb.String()
}

func mergeLabels(sets ...map[string]string) map[string]string {
	out := make(map[string]string)
	for _, s := range sets {
		for k, v := range s {
			out[k] = v
		}
	}
	return out
}

func postJSON(ctx context.Context, url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("received status %d", resp.StatusCode)
	}
	return nil
}
