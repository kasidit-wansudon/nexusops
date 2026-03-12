package activity

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event type constants for the activity feed.
const (
	EventProjectCreated     = "project.created"
	EventProjectUpdated     = "project.updated"
	EventProjectDeleted     = "project.deleted"
	EventDeployStarted      = "deploy.started"
	EventDeployCompleted    = "deploy.completed"
	EventDeployFailed       = "deploy.failed"
	EventPipelineTriggered  = "pipeline.triggered"
	EventPipelineCompleted  = "pipeline.completed"
	EventPipelineFailed     = "pipeline.failed"
	EventMemberAdded        = "member.added"
	EventMemberRemoved      = "member.removed"
	EventMemberRoleChanged  = "member.role_changed"
	EventEnvUpdated         = "env.updated"
	EventEnvCreated         = "env.created"
	EventRollbackTriggered  = "rollback.triggered"
	EventRollbackCompleted  = "rollback.completed"
	EventSettingsChanged    = "settings.changed"
)

// Event represents a single activity or audit log entry.
type Event struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	ActorID      string                 `json:"actor_id"`
	ActorName    string                 `json:"actor_name"`
	ProjectID    string                 `json:"project_id"`
	TeamID       string                 `json:"team_id"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Description  string                 `json:"description"`
	Metadata     map[string]interface{} `json:"metadata"`
	Timestamp    time.Time              `json:"timestamp"`
}

// FilterParams defines criteria for filtering events in the feed.
type FilterParams struct {
	Types     []string  `json:"types"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	ActorID   string    `json:"actor_id"`
	ProjectID string    `json:"project_id"`
	TeamID    string    `json:"team_id"`
}

// subscriber wraps a channel used for real-time event notifications.
type subscriber struct {
	ch        chan *Event
	projectID string
}

// Feed stores events and supports querying, filtering, and real-time
// subscriptions. It uses a bounded ring buffer to cap memory usage.
type Feed struct {
	events      []*Event
	maxSize     int
	subscribers map[string][]*subscriber
	mu          sync.RWMutex
}

// NewFeed creates a new activity feed with the specified maximum event capacity.
func NewFeed(maxSize int) *Feed {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &Feed{
		events:      make([]*Event, 0, maxSize),
		maxSize:     maxSize,
		subscribers: make(map[string][]*subscriber),
	}
}

// Record adds a new event to the feed. If the feed is at capacity, the oldest
// event is evicted. All matching subscribers are notified asynchronously.
func (f *Feed) Record(event *Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	if event.Type == "" {
		return fmt.Errorf("event type cannot be empty")
	}

	f.mu.Lock()

	// Assign an ID and timestamp if not already set.
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}

	// Evict the oldest event if at capacity.
	if len(f.events) >= f.maxSize {
		f.events = f.events[1:]
	}
	f.events = append(f.events, event)

	// Collect subscribers that match this event's project.
	var toNotify []*subscriber
	if event.ProjectID != "" {
		toNotify = append(toNotify, f.subscribers[event.ProjectID]...)
	}
	// Wildcard subscribers receive all events.
	toNotify = append(toNotify, f.subscribers["*"]...)

	f.mu.Unlock()

	// Notify subscribers without holding the lock.
	for _, sub := range toNotify {
		select {
		case sub.ch <- event:
		default:
			// Subscriber channel full; drop to avoid blocking.
		}
	}

	return nil
}

// GetByProject returns the most recent events for a given project, up to limit.
func (f *Feed) GetByProject(projectID string, limit int) []*Event {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.filterLocked(func(e *Event) bool {
		return e.ProjectID == projectID
	}, limit)
}

// GetByTeam returns the most recent events for a given team, up to limit.
func (f *Feed) GetByTeam(teamID string, limit int) []*Event {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.filterLocked(func(e *Event) bool {
		return e.TeamID == teamID
	}, limit)
}

// GetByActor returns the most recent events performed by a given actor, up to limit.
func (f *Feed) GetByActor(actorID string, limit int) []*Event {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.filterLocked(func(e *Event) bool {
		return e.ActorID == actorID
	}, limit)
}

// filterLocked applies a predicate to the event list and returns up to limit
// matching events in reverse chronological order. Must be called with at least
// a read lock held.
func (f *Feed) filterLocked(match func(*Event) bool, limit int) []*Event {
	if limit <= 0 {
		limit = 50
	}

	var results []*Event
	// Iterate in reverse to get the newest events first.
	for i := len(f.events) - 1; i >= 0 && len(results) < limit; i-- {
		if match(f.events[i]) {
			results = append(results, f.events[i])
		}
	}
	return results
}

// Subscribe returns a channel that receives real-time events for the given
// project ID. The returned function must be called to unsubscribe and release
// resources.
func (f *Feed) Subscribe(projectID string) (<-chan *Event, func()) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sub := &subscriber{
		ch:        make(chan *Event, 100),
		projectID: projectID,
	}
	f.subscribers[projectID] = append(f.subscribers[projectID], sub)

	unsubscribe := func() {
		f.mu.Lock()
		defer f.mu.Unlock()

		subs := f.subscribers[projectID]
		for i, s := range subs {
			if s == sub {
				f.subscribers[projectID] = append(subs[:i], subs[i+1:]...)
				close(sub.ch)
				break
			}
		}
	}

	return sub.ch, unsubscribe
}

// Filter returns events matching the given filter parameters.
func (f *Feed) Filter(params FilterParams) []*Event {
	f.mu.RLock()
	defer f.mu.RUnlock()

	typeSet := make(map[string]bool, len(params.Types))
	for _, t := range params.Types {
		typeSet[t] = true
	}

	var results []*Event
	for i := len(f.events) - 1; i >= 0; i-- {
		e := f.events[i]

		if len(typeSet) > 0 && !typeSet[e.Type] {
			continue
		}
		if !params.From.IsZero() && e.Timestamp.Before(params.From) {
			continue
		}
		if !params.To.IsZero() && e.Timestamp.After(params.To) {
			continue
		}
		if params.ActorID != "" && e.ActorID != params.ActorID {
			continue
		}
		if params.ProjectID != "" && e.ProjectID != params.ProjectID {
			continue
		}
		if params.TeamID != "" && e.TeamID != params.TeamID {
			continue
		}

		results = append(results, e)
	}

	return results
}

// Count returns the total number of events currently stored in the feed.
func (f *Feed) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.events)
}

// Recent returns the N most recent events regardless of project or team.
func (f *Feed) Recent(limit int) []*Event {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	start := len(f.events) - limit
	if start < 0 {
		start = 0
	}

	results := make([]*Event, 0, len(f.events)-start)
	for i := len(f.events) - 1; i >= start; i-- {
		results = append(results, f.events[i])
	}
	return results
}
