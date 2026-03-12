package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamerWrite(t *testing.T) {
	s := NewStreamer(100)

	entry := &LogEntry{
		PipelineID: "pipe-1",
		StepName:   "build",
		Line:       "compiling...",
		Stream:     StreamStdout,
	}

	s.Write(entry)

	logs := s.GetLogs("pipe-1")
	require.Len(t, logs, 1)
	assert.Equal(t, "compiling...", logs[0].Line)
	assert.Equal(t, "pipe-1", logs[0].PipelineID)
	assert.Equal(t, "build", logs[0].StepName)
	assert.Equal(t, StreamStdout, logs[0].Stream)
	assert.False(t, logs[0].Timestamp.IsZero(), "Timestamp should be set automatically")
}

func TestStreamerWriteNilIsNoop(t *testing.T) {
	s := NewStreamer(100)
	s.Write(nil) // should not panic
	logs := s.GetLogs("anything")
	assert.Nil(t, logs)
}

func TestStreamerSubscribe(t *testing.T) {
	s := NewStreamer(100)

	// Subscribe before any writes.
	ch, unsub := s.Subscribe("pipe-1")
	defer unsub()

	// Write an entry.
	entry := &LogEntry{
		PipelineID: "pipe-1",
		StepName:   "test",
		Line:       "PASS",
		Timestamp:  time.Now(),
		Stream:     StreamStdout,
	}
	s.Write(entry)

	// Subscriber should receive the entry.
	select {
	case got := <-ch:
		assert.Equal(t, "PASS", got.Line)
		assert.Equal(t, "pipe-1", got.PipelineID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry on subscriber channel")
	}
}

func TestStreamerSubscribeReplay(t *testing.T) {
	s := NewStreamer(100)

	// Write entries before subscribing.
	for i := 0; i < 3; i++ {
		s.Write(&LogEntry{
			PipelineID: "pipe-1",
			StepName:   "setup",
			Line:       "line",
			Timestamp:  time.Now(),
			Stream:     StreamStdout,
		})
	}

	// Subscribe after writes -- should receive buffered entries.
	ch, unsub := s.Subscribe("pipe-1")
	defer unsub()

	received := 0
	for i := 0; i < 3; i++ {
		select {
		case <-ch:
			received++
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for replayed entry")
		}
	}
	assert.Equal(t, 3, received)
}

func TestStreamerGetLogs(t *testing.T) {
	s := NewStreamer(100)

	// No logs initially.
	assert.Nil(t, s.GetLogs("pipe-1"))

	// Write some entries.
	for i := 0; i < 5; i++ {
		s.Write(&LogEntry{
			PipelineID: "pipe-1",
			StepName:   "build",
			Line:       "log line",
			Timestamp:  time.Now(),
			Stream:     StreamStdout,
		})
	}

	logs := s.GetLogs("pipe-1")
	assert.Len(t, logs, 5)

	// Logs for a different pipeline should be empty.
	assert.Nil(t, s.GetLogs("pipe-other"))
}

func TestStreamerCapacity(t *testing.T) {
	bufferSize := 5
	s := NewStreamer(bufferSize)

	// Write more entries than the buffer can hold.
	for i := 0; i < 10; i++ {
		s.Write(&LogEntry{
			PipelineID: "pipe-1",
			StepName:   "build",
			Line:       "line",
			Timestamp:  time.Now(),
			Stream:     StreamStdout,
		})
	}

	// Buffer should have been trimmed to bufferSize.
	logs := s.GetLogs("pipe-1")
	assert.Len(t, logs, bufferSize)
}
