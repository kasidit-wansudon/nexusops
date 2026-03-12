package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAggregator(t *testing.T) {
	a := NewAggregator(100)
	require.NotNil(t, a)
	assert.Equal(t, 0, a.Len())
	assert.NotNil(t, a.buffer)
	assert.NotNil(t, a.streams)
	assert.NotNil(t, a.subscribers)
	assert.Equal(t, 100, a.maxBufferSize)
}

func TestNewAggregator_DefaultSize(t *testing.T) {
	a := NewAggregator(0)
	require.NotNil(t, a)
	assert.Equal(t, 10000, a.maxBufferSize)

	a2 := NewAggregator(-5)
	assert.Equal(t, 10000, a2.maxBufferSize)
}

func TestIngest(t *testing.T) {
	a := NewAggregator(1000)

	now := time.Now()
	entries := []*LogEntry{
		{Timestamp: now, Level: LevelInfo, Service: "api", Message: "request started"},
		{Timestamp: now.Add(time.Second), Level: LevelError, Service: "api", Message: "internal error"},
		{Timestamp: now.Add(2 * time.Second), Level: LevelDebug, Service: "worker", Message: "job queued"},
	}

	for _, e := range entries {
		a.Push(e)
	}

	assert.Equal(t, 3, a.Len())

	// Nil entries should be ignored
	a.Push(nil)
	assert.Equal(t, 3, a.Len())
}

func TestIngest_AutoTimestamp(t *testing.T) {
	a := NewAggregator(100)

	entry := &LogEntry{Level: LevelInfo, Service: "svc", Message: "auto ts"}
	a.Push(entry)

	assert.False(t, entry.Timestamp.IsZero(), "timestamp should be set automatically")
}

func TestQuery(t *testing.T) {
	a := NewAggregator(1000)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := []*LogEntry{
		{Timestamp: base, Level: LevelInfo, Service: "api", Message: "hello world"},
		{Timestamp: base.Add(time.Minute), Level: LevelError, Service: "api", Message: "something failed"},
		{Timestamp: base.Add(2 * time.Minute), Level: LevelWarn, Service: "worker", Message: "slow job"},
		{Timestamp: base.Add(3 * time.Minute), Level: LevelDebug, Service: "api", Message: "debug info"},
	}

	for _, e := range entries {
		a.Push(e)
	}

	// Query all entries (no filters)
	results, err := a.Query(QueryParams{})
	require.NoError(t, err)
	assert.Len(t, results, 4)

	// Query with time range
	results, err = a.Query(QueryParams{
		From: base.Add(30 * time.Second),
		To:   base.Add(2*time.Minute + 30*time.Second),
	})
	require.NoError(t, err)
	assert.Len(t, results, 2) // error and warn entries

	// Query with pattern
	results, err = a.Query(QueryParams{Pattern: "failed"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "something failed", results[0].Message)

	// Query with limit
	results, err = a.Query(QueryParams{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Invalid regex pattern returns error
	_, err = a.Query(QueryParams{Pattern: "[invalid"})
	assert.Error(t, err)
}

func TestQueryByService(t *testing.T) {
	a := NewAggregator(1000)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	a.Push(&LogEntry{Timestamp: base, Level: LevelInfo, Service: "api", Message: "api log 1"})
	a.Push(&LogEntry{Timestamp: base.Add(time.Second), Level: LevelInfo, Service: "worker", Message: "worker log 1"})
	a.Push(&LogEntry{Timestamp: base.Add(2 * time.Second), Level: LevelError, Service: "api", Message: "api log 2"})
	a.Push(&LogEntry{Timestamp: base.Add(3 * time.Second), Level: LevelInfo, Service: "scheduler", Message: "scheduler log 1"})

	// Filter by "api" service
	results, err := a.Query(QueryParams{Service: "api"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, "api", r.Service)
	}

	// Filter by "worker" service
	results, err = a.Query(QueryParams{Service: "worker"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "worker log 1", results[0].Message)

	// Filter by nonexistent service
	results, err = a.Query(QueryParams{Service: "nonexistent"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQueryByLevel(t *testing.T) {
	a := NewAggregator(1000)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	a.Push(&LogEntry{Timestamp: base, Level: LevelInfo, Service: "api", Message: "info msg"})
	a.Push(&LogEntry{Timestamp: base.Add(time.Second), Level: LevelError, Service: "api", Message: "error msg"})
	a.Push(&LogEntry{Timestamp: base.Add(2 * time.Second), Level: LevelWarn, Service: "api", Message: "warn msg"})
	a.Push(&LogEntry{Timestamp: base.Add(3 * time.Second), Level: LevelError, Service: "worker", Message: "another error"})

	// Filter by error level across all services
	results, err := a.Query(QueryParams{Level: LevelError})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, LevelError, r.Level)
	}

	// Filter by warn level
	results, err = a.Query(QueryParams{Level: LevelWarn})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "warn msg", results[0].Message)

	// Combine service + level filter
	results, err = a.Query(QueryParams{Service: "api", Level: LevelError})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "error msg", results[0].Message)
}

func TestAggregatorCapacity(t *testing.T) {
	maxSize := 10
	a := NewAggregator(maxSize)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Push more entries than the buffer size
	for i := 0; i < 15; i++ {
		a.Push(&LogEntry{
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Level:     LevelInfo,
			Service:   "svc",
			Message:   "msg",
		})
	}

	// Buffer should have evicted old entries and not exceed max size
	assert.LessOrEqual(t, a.Len(), maxSize, "buffer should not exceed maxBufferSize")

	// The most recent entries should still be present
	tail := a.Tail("", 5)
	assert.Len(t, tail, 5)
	// The last entry's timestamp should be the most recent one pushed
	assert.Equal(t, base.Add(14*time.Second), tail[len(tail)-1].Timestamp)
}
