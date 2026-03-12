package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Notification status constants.
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// Notification represents a message that can be dispatched across multiple channels.
type Notification struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Channels  []string          `json:"channels"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
	SentAt    time.Time         `json:"sent_at"`
	Status    string            `json:"status"`
}

// Channel defines the interface that all notification backends must implement.
type Channel interface {
	Send(ctx context.Context, n *Notification) error
	Name() string
	Type() string
}

// --- Slack ---

// SlackNotifier sends notifications to a Slack channel via incoming webhook.
type SlackNotifier struct {
	WebhookURL string
	Channel    string
	Username   string
	IconEmoji  string
	client     *http.Client
}

// NewSlackNotifier creates a SlackNotifier with a default HTTP client.
func NewSlackNotifier(webhookURL, channel, username, iconEmoji string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
		Channel:    channel,
		Username:   username,
		IconEmoji:  iconEmoji,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackNotifier) Name() string { return "slack" }
func (s *SlackNotifier) Type() string { return "slack" }

// Send posts a richly formatted message to the configured Slack webhook.
func (s *SlackNotifier) Send(ctx context.Context, n *Notification) error {
	payload := map[string]interface{}{
		"channel":    s.Channel,
		"username":   s.Username,
		"icon_emoji": s.IconEmoji,
		"attachments": []map[string]interface{}{
			{
				"color":  colorForType(n.Type),
				"title":  n.Title,
				"text":   n.Message,
				"ts":     n.CreatedAt.Unix(),
				"fields": metadataToSlackFields(n.Metadata),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func metadataToSlackFields(meta map[string]string) []map[string]interface{} {
	fields := make([]map[string]interface{}, 0, len(meta))
	for k, v := range meta {
		fields = append(fields, map[string]interface{}{
			"title": k,
			"value": v,
			"short": true,
		})
	}
	return fields
}

// --- Discord ---

// DiscordNotifier sends notifications to a Discord channel via webhook.
type DiscordNotifier struct {
	WebhookURL string
	client     *http.Client
}

// NewDiscordNotifier creates a DiscordNotifier with a default HTTP client.
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		WebhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *DiscordNotifier) Name() string { return "discord" }
func (d *DiscordNotifier) Type() string { return "discord" }

// Send posts an embed message to the configured Discord webhook.
func (d *DiscordNotifier) Send(ctx context.Context, n *Notification) error {
	fields := make([]map[string]string, 0, len(n.Metadata))
	for k, v := range n.Metadata {
		fields = append(fields, map[string]string{
			"name":   k,
			"value":  v,
			"inline": "true",
		})
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       n.Title,
				"description": n.Message,
				"color":       discordColorForType(n.Type),
				"timestamp":   n.CreatedAt.Format(time.RFC3339),
				"fields":      fields,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// --- Email ---

// EmailNotifier sends notifications via SMTP email.
type EmailNotifier struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
}

// NewEmailNotifier creates a new EmailNotifier.
func NewEmailNotifier(host string, port int, username, password, from string) *EmailNotifier {
	return &EmailNotifier{
		SMTPHost: host,
		SMTPPort: port,
		Username: username,
		Password: password,
		From:     from,
	}
}

func (e *EmailNotifier) Name() string { return "email" }
func (e *EmailNotifier) Type() string { return "email" }

// Send delivers the notification as an email. The "to" recipient address is
// expected in the notification metadata under the key "email_to".
func (e *EmailNotifier) Send(ctx context.Context, n *Notification) error {
	to := n.Metadata["email_to"]
	if to == "" {
		return fmt.Errorf("email: missing 'email_to' in metadata")
	}

	subject := n.Title
	body := n.Message

	// Build metadata section for the email body.
	var metaLines []string
	for k, v := range n.Metadata {
		if k == "email_to" {
			continue
		}
		metaLines = append(metaLines, fmt.Sprintf("%s: %s", k, v))
	}
	if len(metaLines) > 0 {
		body += "\n\n---\n" + strings.Join(metaLines, "\n")
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		e.From, to, subject, body)

	addr := fmt.Sprintf("%s:%d", e.SMTPHost, e.SMTPPort)

	var auth smtp.Auth
	if e.Username != "" {
		auth = smtp.PlainAuth("", e.Username, e.Password, e.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, e.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("email: send failed: %w", err)
	}
	return nil
}

// --- Webhook ---

// WebhookNotifier sends notifications to an arbitrary HTTP endpoint with an
// optional HMAC-SHA256 signature for payload verification.
type WebhookNotifier struct {
	URL     string
	Secret  string
	Headers map[string]string
	client  *http.Client
}

// NewWebhookNotifier creates a WebhookNotifier.
func NewWebhookNotifier(url, secret string, headers map[string]string) *WebhookNotifier {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &WebhookNotifier{
		URL:     url,
		Secret:  secret,
		Headers: headers,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebhookNotifier) Name() string { return "webhook" }
func (w *WebhookNotifier) Type() string { return "webhook" }

// Send delivers the notification as a JSON POST request. If a secret is
// configured, the payload is signed with HMAC-SHA256 and the signature is
// included in the X-Signature-256 header.
func (w *WebhookNotifier) Send(ctx context.Context, n *Notification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("webhook: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}

	if w.Secret != "" {
		mac := hmac.New(sha256.New, []byte(w.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Signature-256", "sha256="+sig)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// --- Template ---

// Template is a simple notification template with placeholder support.
type Template struct {
	Name    string
	Title   string
	Message string
}

// --- Dispatcher ---

// Dispatcher orchestrates sending notifications through registered channels.
// It supports synchronous and asynchronous delivery with a worker pool.
type Dispatcher struct {
	channels  map[string]Channel
	templates map[string]*Template
	queue     chan *Notification
	history   []*Notification
	mu        sync.RWMutex
	workers   int
}

// NewDispatcher creates a Dispatcher with the specified number of background workers.
func NewDispatcher(workers int) *Dispatcher {
	if workers <= 0 {
		workers = 3
	}
	return &Dispatcher{
		channels:  make(map[string]Channel),
		templates: make(map[string]*Template),
		queue:     make(chan *Notification, 1000),
		history:   make([]*Notification, 0),
		workers:   workers,
	}
}

// RegisterChannel adds a delivery channel to the dispatcher.
func (d *Dispatcher) RegisterChannel(name string, ch Channel) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.channels[name] = ch
}

// RegisterTemplate adds a notification template.
func (d *Dispatcher) RegisterTemplate(name string, t *Template) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.templates[name] = t
}

// Send delivers a notification synchronously to all channels listed in the
// notification's Channels field. It returns the first error encountered.
func (d *Dispatcher) Send(ctx context.Context, n *Notification) error {
	d.prepare(n)

	d.mu.RLock()
	var errs []string
	for _, chName := range n.Channels {
		ch, exists := d.channels[chName]
		if !exists {
			errs = append(errs, fmt.Sprintf("channel %q not registered", chName))
			continue
		}
		if err := ch.Send(ctx, n); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", chName, err))
		}
	}
	d.mu.RUnlock()

	n.SentAt = time.Now()
	if len(errs) > 0 {
		n.Status = StatusFailed
		d.recordHistory(n)
		return fmt.Errorf("notification dispatch errors: %s", strings.Join(errs, "; "))
	}

	n.Status = StatusSent
	d.recordHistory(n)
	return nil
}

// SendAsync queues a notification for asynchronous delivery by the worker pool.
func (d *Dispatcher) SendAsync(n *Notification) {
	d.prepare(n)
	d.queue <- n
}

// Start launches the background worker goroutines that process the async queue.
// It blocks until the provided context is cancelled.
func (d *Dispatcher) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < d.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case n, ok := <-d.queue:
					if !ok {
						return
					}
					_ = d.Send(ctx, n)
				}
			}
		}()
	}
	wg.Wait()
}

// History returns a copy of the notification history.
func (d *Dispatcher) History() []*Notification {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]*Notification, len(d.history))
	copy(result, d.history)
	return result
}

// prepare sets default fields on a notification before sending.
func (d *Dispatcher) prepare(n *Notification) {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	if n.Status == "" {
		n.Status = StatusPending
	}
	if n.Metadata == nil {
		n.Metadata = make(map[string]string)
	}
}

func (d *Dispatcher) recordHistory(n *Notification) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.history = append(d.history, n)
}

// --- Builder helpers ---

// BuildDeployNotification creates a pre-formatted notification for deployment events.
func BuildDeployNotification(project, env, status, version string) *Notification {
	title := fmt.Sprintf("Deployment %s: %s", status, project)
	message := fmt.Sprintf("Deployment of **%s** version **%s** to **%s** environment has %s.",
		project, version, env, status)

	return &Notification{
		ID:   uuid.New().String(),
		Type: "deploy." + status,
		Title:   title,
		Message: message,
		Channels: []string{"slack", "discord"},
		Metadata: map[string]string{
			"project":     project,
			"environment": env,
			"status":      status,
			"version":     version,
		},
		CreatedAt: time.Now(),
		Status:    StatusPending,
	}
}

// BuildPipelineNotification creates a pre-formatted notification for pipeline events.
func BuildPipelineNotification(project, pipeline, status string) *Notification {
	title := fmt.Sprintf("Pipeline %s: %s/%s", status, project, pipeline)
	message := fmt.Sprintf("Pipeline **%s** in project **%s** has %s.",
		pipeline, project, status)

	return &Notification{
		ID:   uuid.New().String(),
		Type: "pipeline." + status,
		Title:   title,
		Message: message,
		Channels: []string{"slack"},
		Metadata: map[string]string{
			"project":  project,
			"pipeline": pipeline,
			"status":   status,
		},
		CreatedAt: time.Now(),
		Status:    StatusPending,
	}
}

// --- Color helpers ---

func colorForType(eventType string) string {
	switch {
	case strings.Contains(eventType, "completed"), strings.Contains(eventType, "success"):
		return "#36a64f"
	case strings.Contains(eventType, "failed"), strings.Contains(eventType, "error"):
		return "#ff0000"
	case strings.Contains(eventType, "started"), strings.Contains(eventType, "triggered"):
		return "#2196f3"
	case strings.Contains(eventType, "rollback"):
		return "#ff9800"
	default:
		return "#808080"
	}
}

func discordColorForType(eventType string) int {
	switch {
	case strings.Contains(eventType, "completed"), strings.Contains(eventType, "success"):
		return 3582783 // green
	case strings.Contains(eventType, "failed"), strings.Contains(eventType, "error"):
		return 16711680 // red
	case strings.Contains(eventType, "started"), strings.Contains(eventType, "triggered"):
		return 2196735 // blue
	case strings.Contains(eventType, "rollback"):
		return 16750848 // orange
	default:
		return 8421504 // gray
	}
}
