package log

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"time"
)

// Stream type constants.
const (
	StreamStdout = "stdout"
	StreamStderr = "stderr"
)

// ANSI colour codes used for log formatting.
const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// LogEntry represents a single log line emitted during pipeline execution.
type LogEntry struct {
	PipelineID string    `json:"pipeline_id"`
	StepName   string    `json:"step_name"`
	Line       string    `json:"line"`
	Timestamp  time.Time `json:"timestamp"`
	Stream     string    `json:"stream"` // "stdout" or "stderr"
}

// subscriber wraps a channel and a done signal so that it can be cleaned up.
type subscriber struct {
	ch   chan *LogEntry
	done chan struct{}
}

// Streamer fans out log entries to subscribers and maintains a per-pipeline
// ring buffer so that late joiners can catch up with recent output.
type Streamer struct {
	bufferSize  int
	subscribers map[string][]*subscriber
	buffer      map[string][]*LogEntry
	mu          sync.RWMutex
}

// NewStreamer creates a Streamer that keeps up to bufferSize log entries per
// pipeline for replay purposes.
func NewStreamer(bufferSize int) *Streamer {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	return &Streamer{
		bufferSize:  bufferSize,
		subscribers: make(map[string][]*subscriber),
		buffer:      make(map[string][]*LogEntry),
	}
}

// Write sends a log entry to all active subscribers for the entry's pipeline
// and appends it to the replay buffer. If a subscriber's channel is full the
// entry is dropped for that subscriber to avoid blocking.
func (s *Streamer) Write(entry *LogEntry) {
	if entry == nil {
		return
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Append to buffer, trimming if necessary.
	buf := s.buffer[entry.PipelineID]
	buf = append(buf, entry)
	if len(buf) > s.bufferSize {
		// Drop oldest entries to stay within the limit.
		excess := len(buf) - s.bufferSize
		buf = buf[excess:]
	}
	s.buffer[entry.PipelineID] = buf

	// Fan out to subscribers.
	subs := s.subscribers[entry.PipelineID]
	activeSubs := subs[:0] // reuse backing array
	for _, sub := range subs {
		select {
		case <-sub.done:
			// Subscriber has been cancelled — skip it.
			continue
		default:
		}
		select {
		case sub.ch <- entry:
		default:
			// Channel full — drop entry for this subscriber.
		}
		activeSubs = append(activeSubs, sub)
	}
	s.subscribers[entry.PipelineID] = activeSubs
}

// Subscribe returns a channel that receives log entries for the given
// pipeline and an unsubscribe function. When done, the caller must invoke
// the returned function to release resources. The channel is pre-filled
// with any buffered entries so the subscriber sees recent history.
func (s *Streamer) Subscribe(pipelineID string) (<-chan *LogEntry, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub := &subscriber{
		ch:   make(chan *LogEntry, s.bufferSize),
		done: make(chan struct{}),
	}

	// Replay buffered entries.
	for _, entry := range s.buffer[pipelineID] {
		select {
		case sub.ch <- entry:
		default:
		}
	}

	s.subscribers[pipelineID] = append(s.subscribers[pipelineID], sub)

	unsubscribe := func() {
		select {
		case <-sub.done:
			return // already unsubscribed
		default:
			close(sub.done)
		}
	}

	return sub.ch, unsubscribe
}

// GetLogs returns a snapshot of all buffered log entries for the given
// pipeline.
func (s *Streamer) GetLogs(pipelineID string) []*LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buf := s.buffer[pipelineID]
	if buf == nil {
		return nil
	}

	// Return a copy to avoid races.
	result := make([]*LogEntry, len(buf))
	copy(result, buf)
	return result
}

// StreamFromReader reads from an io.Reader line-by-line and writes each
// line as a LogEntry. It blocks until the reader is exhausted or an error
// occurs. This is typically used to capture container stdout/stderr.
func (s *Streamer) StreamFromReader(pipelineID, stepName string, reader io.Reader, stream string) {
	scanner := bufio.NewScanner(reader)

	// Allow lines up to 1 MB.
	const maxLineSize = 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	for scanner.Scan() {
		entry := &LogEntry{
			PipelineID: pipelineID,
			StepName:   stepName,
			Line:       scanner.Text(),
			Timestamp:  time.Now(),
			Stream:     stream,
		}
		s.Write(entry)
	}
	// Scanner errors (e.g., line too long) are silently consumed because
	// they are non-fatal for log streaming purposes.
}

// FormatANSI renders a log entry as a coloured string suitable for terminal
// output. The format is:
//
//	[timestamp] step_name | line
//
// Stderr lines are coloured red; timestamps are grey.
func FormatANSI(entry *LogEntry) string {
	ts := entry.Timestamp.Format("15:04:05.000")

	lineColour := ansiReset
	streamIndicator := ""
	switch entry.Stream {
	case StreamStderr:
		lineColour = ansiRed
		streamIndicator = " ERR"
	case StreamStdout:
		lineColour = ansiReset
	}

	return fmt.Sprintf(
		"%s[%s]%s %s%s%s%s | %s%s%s",
		ansiGray, ts, ansiReset,
		ansiCyan, entry.StepName, streamIndicator, ansiReset,
		lineColour, entry.Line, ansiReset,
	)
}

// FormatPlain renders a log entry without ANSI escape codes.
func FormatPlain(entry *LogEntry) string {
	ts := entry.Timestamp.Format("15:04:05.000")
	return fmt.Sprintf("[%s] %s | %s", ts, entry.StepName, entry.Line)
}

// Flush clears all buffered log entries and removes all subscribers for the
// given pipeline. Active subscriber channels are closed via the done signal.
func (s *Streamer) Flush(pipelineID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Signal all subscribers to stop.
	for _, sub := range s.subscribers[pipelineID] {
		select {
		case <-sub.done:
		default:
			close(sub.done)
		}
	}

	delete(s.subscribers, pipelineID)
	delete(s.buffer, pipelineID)
}
