package activity

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewFeed(t *testing.T) {
	feed := NewFeed(100)
	if feed == nil {
		t.Fatal("NewFeed returned nil")
	}
	if feed.maxSize != 100 {
		t.Errorf("maxSize = %d, want 100", feed.maxSize)
	}
	if feed.Count() != 0 {
		t.Errorf("new feed should have 0 events, got %d", feed.Count())
	}

	// Zero or negative maxSize should default to 10000.
	feedDefault := NewFeed(0)
	if feedDefault.maxSize != 10000 {
		t.Errorf("maxSize for 0 input = %d, want 10000", feedDefault.maxSize)
	}
	feedNeg := NewFeed(-5)
	if feedNeg.maxSize != 10000 {
		t.Errorf("maxSize for negative input = %d, want 10000", feedNeg.maxSize)
	}
}

func TestAddActivity(t *testing.T) {
	feed := NewFeed(100)

	event := &Event{
		Type:        EventProjectCreated,
		ActorID:     "user-1",
		ActorName:   "Alice",
		ProjectID:   "proj-1",
		TeamID:      "team-1",
		Description: "Created project Alpha",
	}

	err := feed.Record(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ID == "" {
		t.Error("ID should be assigned by Record")
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be assigned by Record")
	}
	if event.Metadata == nil {
		t.Error("Metadata should be initialized by Record")
	}
	if feed.Count() != 1 {
		t.Errorf("feed count = %d, want 1", feed.Count())
	}

	// Nil event should return error.
	err = feed.Record(nil)
	if err == nil {
		t.Fatal("expected error for nil event but got nil")
	}
	if !strings.Contains(err.Error(), "cannot be nil") {
		t.Errorf("error %q should contain 'cannot be nil'", err.Error())
	}

	// Empty type should return error.
	err = feed.Record(&Event{Type: ""})
	if err == nil {
		t.Fatal("expected error for empty type but got nil")
	}
	if !strings.Contains(err.Error(), "type cannot be empty") {
		t.Errorf("error %q should contain 'type cannot be empty'", err.Error())
	}
}

func TestRecordPreservesExistingIDAndTimestamp(t *testing.T) {
	feed := NewFeed(100)

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	event := &Event{
		ID:        "custom-id",
		Type:      EventDeployStarted,
		Timestamp: ts,
	}

	err := feed.Record(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ID != "custom-id" {
		t.Errorf("ID = %q, should not overwrite existing ID", event.ID)
	}
	if !event.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, should not overwrite existing Timestamp", event.Timestamp)
	}
}

func TestGetActivities(t *testing.T) {
	feed := NewFeed(100)

	for i := 0; i < 5; i++ {
		err := feed.Record(&Event{
			Type:        EventPipelineTriggered,
			Description: fmt.Sprintf("event-%d", i),
		})
		if err != nil {
			t.Fatalf("unexpected error recording event %d: %v", i, err)
		}
	}

	recent := feed.Recent(5)
	if len(recent) != 5 {
		t.Fatalf("expected 5 recent events, got %d", len(recent))
	}

	// Recent should return in reverse chronological order (newest first).
	if recent[0].Description != "event-4" {
		t.Errorf("first recent event = %q, want %q", recent[0].Description, "event-4")
	}
	if recent[4].Description != "event-0" {
		t.Errorf("last recent event = %q, want %q", recent[4].Description, "event-0")
	}

	// Requesting more than available should return all.
	all := feed.Recent(100)
	if len(all) != 5 {
		t.Errorf("expected 5 events when requesting 100, got %d", len(all))
	}

	// Default limit (0 or negative) should return up to 50.
	defaultRecent := feed.Recent(0)
	if len(defaultRecent) != 5 {
		t.Errorf("expected 5 events with default limit, got %d", len(defaultRecent))
	}
}

func TestGetByProject(t *testing.T) {
	feed := NewFeed(100)

	// Add events for different projects.
	for i := 0; i < 5; i++ {
		if err := feed.Record(&Event{
			Type:      EventProjectUpdated,
			ProjectID: "proj-1",
			ActorID:   "user-1",
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := feed.Record(&Event{
			Type:      EventProjectUpdated,
			ProjectID: "proj-2",
			ActorID:   "user-2",
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	results := feed.GetByProject("proj-1", 50)
	if len(results) != 5 {
		t.Errorf("expected 5 events for proj-1, got %d", len(results))
	}
	for _, e := range results {
		if e.ProjectID != "proj-1" {
			t.Errorf("event ProjectID = %q, want %q", e.ProjectID, "proj-1")
		}
	}

	results = feed.GetByProject("proj-2", 50)
	if len(results) != 3 {
		t.Errorf("expected 3 events for proj-2, got %d", len(results))
	}

	results = feed.GetByProject("proj-nonexistent", 50)
	if len(results) != 0 {
		t.Errorf("expected 0 events for nonexistent project, got %d", len(results))
	}

	// Respect limit.
	results = feed.GetByProject("proj-1", 2)
	if len(results) != 2 {
		t.Errorf("expected 2 events with limit=2, got %d", len(results))
	}
}

func TestGetByUser(t *testing.T) {
	feed := NewFeed(100)

	if err := feed.Record(&Event{Type: EventDeployStarted, ActorID: "actor-a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := feed.Record(&Event{Type: EventDeployCompleted, ActorID: "actor-a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := feed.Record(&Event{Type: EventDeployStarted, ActorID: "actor-b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := feed.GetByActor("actor-a", 50)
	if len(results) != 2 {
		t.Errorf("expected 2 events for actor-a, got %d", len(results))
	}
	for _, e := range results {
		if e.ActorID != "actor-a" {
			t.Errorf("event ActorID = %q, want %q", e.ActorID, "actor-a")
		}
	}

	results = feed.GetByActor("actor-b", 50)
	if len(results) != 1 {
		t.Errorf("expected 1 event for actor-b, got %d", len(results))
	}

	results = feed.GetByActor("nonexistent", 50)
	if len(results) != 0 {
		t.Errorf("expected 0 events for nonexistent actor, got %d", len(results))
	}
}

func TestFeedCapacity(t *testing.T) {
	maxSize := 5
	feed := NewFeed(maxSize)

	// Record more events than the capacity.
	for i := 0; i < 10; i++ {
		err := feed.Record(&Event{
			Type:        EventPipelineTriggered,
			Description: fmt.Sprintf("event-%d", i),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if feed.Count() != maxSize {
		t.Errorf("feed count = %d, want %d (maxSize)", feed.Count(), maxSize)
	}

	// The oldest events (0-4) should be evicted; the newest (5-9) should remain.
	recent := feed.Recent(maxSize)
	if len(recent) != maxSize {
		t.Fatalf("expected %d recent events, got %d", maxSize, len(recent))
	}
	if recent[0].Description != "event-9" {
		t.Errorf("newest event = %q, want %q", recent[0].Description, "event-9")
	}
	if recent[maxSize-1].Description != "event-5" {
		t.Errorf("oldest event = %q, want %q", recent[maxSize-1].Description, "event-5")
	}
}

func TestFilter(t *testing.T) {
	feed := NewFeed(100)

	now := time.Now()
	feed.Record(&Event{Type: EventDeployStarted, ProjectID: "proj-1", ActorID: "user-1", Timestamp: now.Add(-2 * time.Hour)})
	feed.Record(&Event{Type: EventDeployCompleted, ProjectID: "proj-1", ActorID: "user-2", Timestamp: now.Add(-1 * time.Hour)})
	feed.Record(&Event{Type: EventDeployFailed, ProjectID: "proj-2", ActorID: "user-1", Timestamp: now})
	feed.Record(&Event{Type: EventProjectCreated, ProjectID: "proj-1", ActorID: "user-1", TeamID: "team-1", Timestamp: now.Add(-30 * time.Minute)})

	// Filter by type.
	results := feed.Filter(FilterParams{
		Types: []string{EventDeployStarted, EventDeployCompleted, EventDeployFailed},
	})
	if len(results) != 3 {
		t.Errorf("expected 3 deploy events, got %d", len(results))
	}

	// Filter by actor.
	results = feed.Filter(FilterParams{ActorID: "user-1"})
	if len(results) != 3 {
		t.Errorf("expected 3 events by user-1, got %d", len(results))
	}

	// Filter by project and type.
	results = feed.Filter(FilterParams{
		Types:     []string{EventDeployStarted},
		ProjectID: "proj-1",
	})
	if len(results) != 1 {
		t.Errorf("expected 1 deploy.started event for proj-1, got %d", len(results))
	}

	// Filter by time range.
	results = feed.Filter(FilterParams{
		From: now.Add(-90 * time.Minute),
	})
	if len(results) != 3 {
		t.Errorf("expected 3 events from last 90 minutes, got %d", len(results))
	}

	// Filter by team.
	results = feed.Filter(FilterParams{TeamID: "team-1"})
	if len(results) != 1 {
		t.Errorf("expected 1 event for team-1, got %d", len(results))
	}
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	feed := NewFeed(100)

	ch, unsub := feed.Subscribe("proj-1")

	// Record an event for the subscribed project.
	err := feed.Record(&Event{
		Type:      EventDeployStarted,
		ProjectID: "proj-1",
		ActorID:   "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The subscriber should receive the event.
	select {
	case evt := <-ch:
		if evt.Type != EventDeployStarted {
			t.Errorf("event type = %q, want %q", evt.Type, EventDeployStarted)
		}
		if evt.ProjectID != "proj-1" {
			t.Errorf("event ProjectID = %q, want %q", evt.ProjectID, "proj-1")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("subscriber did not receive event within timeout")
	}

	// Events for other projects should not be received.
	err = feed.Record(&Event{
		Type:      EventDeployStarted,
		ProjectID: "proj-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-ch:
		t.Fatal("subscriber should not receive events for other projects")
	case <-time.After(50 * time.Millisecond):
		// Expected: no event received.
	}

	// Unsubscribe and verify channel is closed.
	unsub()

	err = feed.Record(&Event{
		Type:      EventDeployCompleted,
		ProjectID: "proj-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
		// Also acceptable: no message.
	}
}
