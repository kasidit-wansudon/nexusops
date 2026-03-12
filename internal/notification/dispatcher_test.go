package notification

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// --- test double ---

type fakeChannel struct {
	name    string
	sent    []*Notification
	mu      sync.Mutex
	sendErr error
}

func newFakeChannel(name string) *fakeChannel {
	return &fakeChannel{name: name}
}

func (f *fakeChannel) Send(_ context.Context, n *Notification) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, n)
	return nil
}

func (f *fakeChannel) Name() string { return f.name }
func (f *fakeChannel) Type() string { return f.name }

func (f *fakeChannel) sentCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

// --- tests ---

func TestNewDispatcher(t *testing.T) {
	tests := []struct {
		name            string
		workers         int
		expectedWorkers int
	}{
		{"positive workers", 5, 5},
		{"zero workers defaults to 3", 0, 3},
		{"negative workers defaults to 3", -1, 3},
		{"one worker", 1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDispatcher(tc.workers)
			if d == nil {
				t.Fatal("NewDispatcher returned nil")
			}
			if d.channels == nil {
				t.Fatal("channels map is nil")
			}
			if d.templates == nil {
				t.Fatal("templates map is nil")
			}
			if d.queue == nil {
				t.Fatal("queue channel is nil")
			}
			if d.history == nil {
				t.Fatal("history slice is nil")
			}
			if d.workers != tc.expectedWorkers {
				t.Errorf("workers = %d, want %d", d.workers, tc.expectedWorkers)
			}
		})
	}
}

func TestDispatcherSend(t *testing.T) {
	d := NewDispatcher(1)
	ch := newFakeChannel("slack")
	d.RegisterChannel("slack", ch)

	ctx := context.Background()
	n := &Notification{
		Title:    "test notification",
		Message:  "hello world",
		Channels: []string{"slack"},
	}
	err := d.Send(ctx, n)
	if err != nil {
		t.Fatalf("Send returned unexpected error: %v", err)
	}

	// Verify the notification was delivered to the channel.
	if ch.sentCount() != 1 {
		t.Errorf("sentCount = %d, want 1", ch.sentCount())
	}

	// Verify defaults were set by prepare().
	if n.ID == "" {
		t.Error("ID was not set by prepare()")
	}
	if n.CreatedAt.IsZero() {
		t.Error("CreatedAt was not set by prepare()")
	}
	if n.Status != StatusSent {
		t.Errorf("Status = %q, want %q", n.Status, StatusSent)
	}
	if n.Metadata == nil {
		t.Error("Metadata was not initialized by prepare()")
	}
	if n.SentAt.IsZero() {
		t.Error("SentAt was not set after sending")
	}
}

func TestDispatcherSendUnregisteredChannel(t *testing.T) {
	d := NewDispatcher(1)

	ctx := context.Background()
	n := &Notification{
		Title:    "test",
		Channels: []string{"nonexistent"},
	}
	err := d.Send(ctx, n)
	if err == nil {
		t.Fatal("expected error for unregistered channel, got nil")
	}
	if got := err.Error(); !contains(got, "not registered") {
		t.Errorf("error = %q, want it to contain 'not registered'", got)
	}

	history := d.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].Status != StatusFailed {
		t.Errorf("Status = %q, want %q", history[0].Status, StatusFailed)
	}
}

func TestDispatcherSendChannelError(t *testing.T) {
	d := NewDispatcher(1)
	ch := newFakeChannel("broken")
	ch.sendErr = fmt.Errorf("connection refused")
	d.RegisterChannel("broken", ch)

	ctx := context.Background()
	n := &Notification{
		Title:    "test",
		Channels: []string{"broken"},
	}
	err := d.Send(ctx, n)
	if err == nil {
		t.Fatal("expected error when channel returns error, got nil")
	}
	if !contains(err.Error(), "connection refused") {
		t.Errorf("error = %q, want it to contain 'connection refused'", err.Error())
	}
	if n.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", n.Status, StatusFailed)
	}
}

func TestDispatcherRegisterChannelAndHistory(t *testing.T) {
	d := NewDispatcher(1)
	ch1 := newFakeChannel("slack")
	ch2 := newFakeChannel("discord")
	d.RegisterChannel("slack", ch1)
	d.RegisterChannel("discord", ch2)

	ctx := context.Background()

	// Send to both channels.
	n := &Notification{
		Title:    "Deploy started",
		Message:  "Starting deploy",
		Channels: []string{"slack", "discord"},
	}
	if err := d.Send(ctx, n); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	// Both channels should have received the notification.
	if ch1.sentCount() != 1 {
		t.Errorf("slack sentCount = %d, want 1", ch1.sentCount())
	}
	if ch2.sentCount() != 1 {
		t.Errorf("discord sentCount = %d, want 1", ch2.sentCount())
	}

	// History should record the notification.
	history := d.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].Status != StatusSent {
		t.Errorf("Status = %q, want %q", history[0].Status, StatusSent)
	}
}

func TestBuildDeployNotification(t *testing.T) {
	n := BuildDeployNotification("myapp", "production", "success", "v2.1.0")
	if n == nil {
		t.Fatal("BuildDeployNotification returned nil")
	}
	if n.ID == "" {
		t.Error("ID is empty")
	}
	if n.Type != "deploy.success" {
		t.Errorf("Type = %q, want %q", n.Type, "deploy.success")
	}
	if !contains(n.Title, "success") || !contains(n.Title, "myapp") {
		t.Errorf("Title = %q, want it to contain 'success' and 'myapp'", n.Title)
	}
	if !contains(n.Message, "v2.1.0") || !contains(n.Message, "production") {
		t.Errorf("Message = %q, want it to contain 'v2.1.0' and 'production'", n.Message)
	}
	if len(n.Channels) != 2 || n.Channels[0] != "slack" || n.Channels[1] != "discord" {
		t.Errorf("Channels = %v, want [slack discord]", n.Channels)
	}
	if n.Status != StatusPending {
		t.Errorf("Status = %q, want %q", n.Status, StatusPending)
	}
	if n.Metadata["project"] != "myapp" {
		t.Errorf("Metadata[project] = %q, want %q", n.Metadata["project"], "myapp")
	}
	if n.Metadata["environment"] != "production" {
		t.Errorf("Metadata[environment] = %q, want %q", n.Metadata["environment"], "production")
	}
}

func TestBuildPipelineNotification(t *testing.T) {
	n := BuildPipelineNotification("myapp", "ci-pipeline", "failed")
	if n == nil {
		t.Fatal("BuildPipelineNotification returned nil")
	}
	if n.Type != "pipeline.failed" {
		t.Errorf("Type = %q, want %q", n.Type, "pipeline.failed")
	}
	if !contains(n.Title, "failed") || !contains(n.Title, "ci-pipeline") {
		t.Errorf("Title = %q, want it to contain 'failed' and 'ci-pipeline'", n.Title)
	}
	if len(n.Channels) != 1 || n.Channels[0] != "slack" {
		t.Errorf("Channels = %v, want [slack]", n.Channels)
	}
	if n.Metadata["pipeline"] != "ci-pipeline" {
		t.Errorf("Metadata[pipeline] = %q, want %q", n.Metadata["pipeline"], "ci-pipeline")
	}
}

// contains is a small helper to avoid importing strings in tests.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
