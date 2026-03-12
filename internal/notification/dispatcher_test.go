package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChannel implements Channel for testing.
type mockChannel struct {
	name     string
	typ      string
	sent     []*Notification
	sendErr  error
}

func (m *mockChannel) Name() string { return m.name }
func (m *mockChannel) Type() string { return m.typ }
func (m *mockChannel) Send(_ context.Context, n *Notification) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, n)
	return nil
}

func TestNewDispatcherDefaultWorkers(t *testing.T) {
	d := NewDispatcher(0)
	assert.Equal(t, 3, d.workers)
}

func TestDispatcherRegisterChannel(t *testing.T) {
	d := NewDispatcher(1)
	ch := &mockChannel{name: "test", typ: "test"}
	d.RegisterChannel("test", ch)
	assert.Contains(t, d.channels, "test")
}

func TestDispatcherSendSuccess(t *testing.T) {
	d := NewDispatcher(1)
	ch := &mockChannel{name: "mock", typ: "mock"}
	d.RegisterChannel("mock", ch)

	n := &Notification{
		Title:    "Test",
		Message:  "Hello",
		Channels: []string{"mock"},
	}

	err := d.Send(context.Background(), n)
	require.NoError(t, err)
	assert.Len(t, ch.sent, 1)
	assert.Equal(t, StatusSent, n.Status)
	assert.NotEmpty(t, n.ID)
}

func TestDispatcherSendUnregisteredChannel(t *testing.T) {
	d := NewDispatcher(1)

	n := &Notification{
		Title:    "Test",
		Message:  "Hello",
		Channels: []string{"nonexistent"},
	}

	err := d.Send(context.Background(), n)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
	assert.Equal(t, StatusFailed, n.Status)
}

func TestDispatcherSendChannelError(t *testing.T) {
	d := NewDispatcher(1)
	ch := &mockChannel{name: "fail", typ: "fail", sendErr: assert.AnError}
	d.RegisterChannel("fail", ch)

	n := &Notification{
		Title:    "Test",
		Message:  "Hello",
		Channels: []string{"fail"},
	}

	err := d.Send(context.Background(), n)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, n.Status)
}

func TestDispatcherHistory(t *testing.T) {
	d := NewDispatcher(1)
	ch := &mockChannel{name: "mock", typ: "mock"}
	d.RegisterChannel("mock", ch)

	d.Send(context.Background(), &Notification{Channels: []string{"mock"}, Title: "A"})
	d.Send(context.Background(), &Notification{Channels: []string{"mock"}, Title: "B"})

	history := d.History()
	assert.Len(t, history, 2)
}

func TestDispatcherPrepare(t *testing.T) {
	d := NewDispatcher(1)
	ch := &mockChannel{name: "mock", typ: "mock"}
	d.RegisterChannel("mock", ch)

	n := &Notification{Channels: []string{"mock"}, Title: "Test"}
	d.Send(context.Background(), n)

	assert.NotEmpty(t, n.ID)
	assert.False(t, n.CreatedAt.IsZero())
	assert.NotNil(t, n.Metadata)
}

func TestDispatcherRegisterTemplate(t *testing.T) {
	d := NewDispatcher(1)
	tmpl := &Template{Name: "deploy", Title: "Deploy {{project}}", Message: "Deploying..."}
	d.RegisterTemplate("deploy", tmpl)
	assert.Contains(t, d.templates, "deploy")
}

func TestBuildDeployNotification(t *testing.T) {
	n := BuildDeployNotification("myapp", "production", "succeeded", "v1.2.3")
	assert.Contains(t, n.Title, "myapp")
	assert.Contains(t, n.Message, "production")
	assert.Contains(t, n.Message, "v1.2.3")
	assert.Equal(t, "deploy.succeeded", n.Type)
	assert.Contains(t, n.Channels, "slack")
	assert.Contains(t, n.Channels, "discord")
	assert.Equal(t, "myapp", n.Metadata["project"])
}

func TestBuildPipelineNotification(t *testing.T) {
	n := BuildPipelineNotification("myapp", "ci", "failed")
	assert.Contains(t, n.Title, "ci")
	assert.Contains(t, n.Title, "failed")
	assert.Equal(t, "pipeline.failed", n.Type)
	assert.Contains(t, n.Channels, "slack")
}

func TestColorForType(t *testing.T) {
	assert.Equal(t, "#36a64f", colorForType("deploy.completed"))
	assert.Equal(t, "#ff0000", colorForType("pipeline.failed"))
	assert.Equal(t, "#2196f3", colorForType("deploy.started"))
	assert.Equal(t, "#ff9800", colorForType("rollback"))
	assert.Equal(t, "#808080", colorForType("unknown"))
}

func TestDiscordColorForType(t *testing.T) {
	assert.Equal(t, 3582783, discordColorForType("deploy.completed"))
	assert.Equal(t, 16711680, discordColorForType("pipeline.failed"))
	assert.Equal(t, 2196735, discordColorForType("deploy.started"))
	assert.Equal(t, 16750848, discordColorForType("rollback"))
	assert.Equal(t, 8421504, discordColorForType("unknown"))
}

func TestWebhookNotifierSend(t *testing.T) {
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wh := NewWebhookNotifier(server.URL, "secret123", nil)

	n := &Notification{
		ID:        "n1",
		Title:     "Test",
		Message:   "Hello",
		CreatedAt: time.Now(),
		Metadata:  map[string]string{},
	}

	err := wh.Send(context.Background(), n)
	require.NoError(t, err)
	assert.Contains(t, receivedSig, "sha256=")
}

func TestWebhookNotifierNoSecret(t *testing.T) {
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wh := NewWebhookNotifier(server.URL, "", nil)
	n := &Notification{ID: "n1", Title: "Test", Metadata: map[string]string{}, CreatedAt: time.Now()}
	err := wh.Send(context.Background(), n)
	require.NoError(t, err)
	assert.Empty(t, receivedSig)
}

func TestWebhookNotifierServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	wh := NewWebhookNotifier(server.URL, "", nil)
	n := &Notification{ID: "n1", Title: "Test", Metadata: map[string]string{}, CreatedAt: time.Now()}
	err := wh.Send(context.Background(), n)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSlackNotifierInterface(t *testing.T) {
	s := NewSlackNotifier("https://hooks.slack.com/test", "#general", "bot", ":robot:")
	assert.Equal(t, "slack", s.Name())
	assert.Equal(t, "slack", s.Type())
}

func TestDiscordNotifierInterface(t *testing.T) {
	d := NewDiscordNotifier("https://discord.com/api/webhooks/test")
	assert.Equal(t, "discord", d.Name())
	assert.Equal(t, "discord", d.Type())
}

func TestEmailNotifierInterface(t *testing.T) {
	e := NewEmailNotifier("smtp.example.com", 587, "user", "pass", "noreply@example.com")
	assert.Equal(t, "email", e.Name())
	assert.Equal(t, "email", e.Type())
}

func TestWebhookNotifierCustomHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("X-Custom-Auth")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wh := NewWebhookNotifier(server.URL, "", map[string]string{"X-Custom-Auth": "token123"})
	n := &Notification{ID: "n1", Title: "Test", Metadata: map[string]string{}, CreatedAt: time.Now()}
	err := wh.Send(context.Background(), n)
	require.NoError(t, err)
	assert.Equal(t, "token123", receivedAuth)
}
