package activity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFeedDefaultSize(t *testing.T) {
	f := NewFeed(0)
	assert.Equal(t, 10000, f.maxSize)
}

func TestRecordAndCount(t *testing.T) {
	f := NewFeed(100)
	err := f.Record(&Event{Type: EventProjectCreated, ProjectID: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 1, f.Count())
}

func TestRecordNilEvent(t *testing.T) {
	f := NewFeed(100)
	err := f.Record(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestRecordEmptyType(t *testing.T) {
	f := NewFeed(100)
	err := f.Record(&Event{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
}

func TestRecordAssignsIDAndTimestamp(t *testing.T) {
	f := NewFeed(100)
	e := &Event{Type: EventDeployStarted}
	f.Record(e)
	assert.NotEmpty(t, e.ID)
	assert.False(t, e.Timestamp.IsZero())
}

func TestRecordEviction(t *testing.T) {
	f := NewFeed(3)
	for i := 0; i < 5; i++ {
		f.Record(&Event{Type: EventProjectCreated})
	}
	assert.Equal(t, 3, f.Count())
}

func TestGetByProject(t *testing.T) {
	f := NewFeed(100)
	f.Record(&Event{Type: EventDeployStarted, ProjectID: "p1"})
	f.Record(&Event{Type: EventDeployStarted, ProjectID: "p2"})
	f.Record(&Event{Type: EventDeployCompleted, ProjectID: "p1"})

	events := f.GetByProject("p1", 10)
	assert.Len(t, events, 2)
}

func TestGetByTeam(t *testing.T) {
	f := NewFeed(100)
	f.Record(&Event{Type: EventMemberAdded, TeamID: "t1"})
	f.Record(&Event{Type: EventMemberAdded, TeamID: "t2"})

	events := f.GetByTeam("t1", 10)
	assert.Len(t, events, 1)
}

func TestGetByActor(t *testing.T) {
	f := NewFeed(100)
	f.Record(&Event{Type: EventEnvUpdated, ActorID: "a1"})
	f.Record(&Event{Type: EventEnvCreated, ActorID: "a2"})

	events := f.GetByActor("a1", 10)
	assert.Len(t, events, 1)
}

func TestRecent(t *testing.T) {
	f := NewFeed(100)
	for i := 0; i < 10; i++ {
		f.Record(&Event{Type: EventProjectCreated})
	}

	events := f.Recent(3)
	assert.Len(t, events, 3)
}

func TestRecentDefaultLimit(t *testing.T) {
	f := NewFeed(100)
	for i := 0; i < 5; i++ {
		f.Record(&Event{Type: EventProjectCreated})
	}

	events := f.Recent(0)
	assert.Len(t, events, 5) // less than default 50
}

func TestSubscribe(t *testing.T) {
	f := NewFeed(100)
	ch, unsub := f.Subscribe("p1")
	defer unsub()

	f.Record(&Event{Type: EventDeployStarted, ProjectID: "p1"})

	select {
	case e := <-ch:
		assert.Equal(t, EventDeployStarted, e.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSubscribeNoMatchingProject(t *testing.T) {
	f := NewFeed(100)
	ch, unsub := f.Subscribe("p1")
	defer unsub()

	f.Record(&Event{Type: EventDeployStarted, ProjectID: "p2"})

	select {
	case <-ch:
		t.Fatal("should not receive event for different project")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestSubscribeWildcard(t *testing.T) {
	f := NewFeed(100)
	ch, unsub := f.Subscribe("*")
	defer unsub()

	f.Record(&Event{Type: EventDeployStarted, ProjectID: "p1"})

	select {
	case e := <-ch:
		assert.Equal(t, EventDeployStarted, e.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for wildcard event")
	}
}

func TestFilter(t *testing.T) {
	f := NewFeed(100)
	now := time.Now()
	f.Record(&Event{Type: EventDeployStarted, ProjectID: "p1", ActorID: "a1", TeamID: "t1", Timestamp: now})
	f.Record(&Event{Type: EventDeployCompleted, ProjectID: "p1", ActorID: "a2", TeamID: "t1"})
	f.Record(&Event{Type: EventMemberAdded, ProjectID: "p2", ActorID: "a1", TeamID: "t2"})

	// Filter by type
	results := f.Filter(FilterParams{Types: []string{EventDeployStarted}})
	assert.Len(t, results, 1)

	// Filter by actor
	results = f.Filter(FilterParams{ActorID: "a1"})
	assert.Len(t, results, 2)

	// Filter by project
	results = f.Filter(FilterParams{ProjectID: "p1"})
	assert.Len(t, results, 2)

	// Filter by team
	results = f.Filter(FilterParams{TeamID: "t1"})
	assert.Len(t, results, 2)
}

func TestFilterByTimeRange(t *testing.T) {
	f := NewFeed(100)
	past := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-30 * time.Minute)

	f.Record(&Event{Type: EventProjectCreated, Timestamp: past})
	f.Record(&Event{Type: EventProjectUpdated, Timestamp: recent})

	results := f.Filter(FilterParams{
		From: time.Now().Add(-1 * time.Hour),
	})
	assert.Len(t, results, 1)
	assert.Equal(t, EventProjectUpdated, results[0].Type)
}
