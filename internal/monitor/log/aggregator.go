// Package log provides a log aggregation system with in-memory buffering,
// streaming subscriptions, query filtering, and Loki-compatible export.
package log

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Level represents the severity of a log entry.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// LogEntry represents a single structured log line.
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     Level                  `json:"level"`
	Service   string                 `json:"service"`
	Message   string                 `json:"message"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// QueryParams defines the filters applied when querying stored logs.
type QueryParams struct {
	Service string
	Level   Level
	From    time.Time
	To      time.Time
	Limit   int
	Pattern string // regex pattern matched against Message
}

// subscriber represents a live log stream consumer.
type subscriber struct {
	ch     chan *LogEntry
	cancel chan struct{}
}

// Aggregator collects, stores, and streams log entries. It maintains a
// bounded in-memory buffer organised by service.
type Aggregator struct {
	mu            sync.RWMutex
	buffer        []*LogEntry
	streams       map[string][]*LogEntry // keyed by service
	maxBufferSize int

	subMu       sync.RWMutex
	subscribers map[string][]*subscriber // keyed by service ("" = all)
}

// NewAggregator creates an Aggregator that retains at most maxBufferSize
// entries in its global buffer. Per-service streams share the same limit.
func NewAggregator(maxBufferSize int) *Aggregator {
	if maxBufferSize <= 0 {
		maxBufferSize = 10000
	}
	return &Aggregator{
		buffer:        make([]*LogEntry, 0, maxBufferSize),
		streams:       make(map[string][]*LogEntry),
		maxBufferSize: maxBufferSize,
		subscribers:   make(map[string][]*subscriber),
	}
}

// Push appends a log entry to the global buffer and the appropriate service
// stream, trimming the oldest entries when the buffer is full. It also fans
// out the entry to any active subscribers.
func (a *Aggregator) Push(entry *LogEntry) {
	if entry == nil {
		return
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	a.mu.Lock()

	// Append to global buffer with eviction.
	if len(a.buffer) >= a.maxBufferSize {
		// Drop the oldest 10% to amortise eviction cost.
		drop := a.maxBufferSize / 10
		if drop == 0 {
			drop = 1
		}
		a.buffer = a.buffer[drop:]
	}
	a.buffer = append(a.buffer, entry)

	// Append to per-service stream.
	svc := entry.Service
	stream := a.streams[svc]
	if len(stream) >= a.maxBufferSize {
		drop := a.maxBufferSize / 10
		if drop == 0 {
			drop = 1
		}
		stream = stream[drop:]
	}
	a.streams[svc] = append(stream, entry)

	a.mu.Unlock()

	// Notify subscribers.
	a.fanOut(entry)
}

// fanOut sends the entry to matching subscribers without holding the main lock.
func (a *Aggregator) fanOut(entry *LogEntry) {
	a.subMu.RLock()
	defer a.subMu.RUnlock()

	deliver := func(subs []*subscriber) {
		for _, s := range subs {
			select {
			case s.ch <- entry:
			default:
				// Slow consumer — drop to avoid blocking.
			}
		}
	}

	// Subscribers registered for this specific service.
	if subs, ok := a.subscribers[entry.Service]; ok {
		deliver(subs)
	}
	// Subscribers registered for all services (empty key).
	if subs, ok := a.subscribers[""]; ok {
		deliver(subs)
	}
}

// Query returns log entries matching the given parameters. Results are sorted
// oldest-first and capped at params.Limit.
func (a *Aggregator) Query(params QueryParams) ([]*LogEntry, error) {
	var re *regexp.Regexp
	if params.Pattern != "" {
		var err error
		re, err = regexp.Compile(params.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	// Decide source slice.
	var source []*LogEntry
	if params.Service != "" {
		source = a.streams[params.Service]
	} else {
		source = a.buffer
	}

	var results []*LogEntry
	for _, e := range source {
		if !params.From.IsZero() && e.Timestamp.Before(params.From) {
			continue
		}
		if !params.To.IsZero() && e.Timestamp.After(params.To) {
			continue
		}
		if params.Level != "" && e.Level != params.Level {
			continue
		}
		if re != nil && !re.MatchString(e.Message) {
			continue
		}
		results = append(results, e)
	}

	// Sort by timestamp ascending.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	if params.Limit > 0 && len(results) > params.Limit {
		results = results[len(results)-params.Limit:]
	}

	return results, nil
}

// Subscribe returns a channel that receives new log entries for the given
// service (or all services if service is ""). The returned function must be
// called to unsubscribe and free resources.
func (a *Aggregator) Subscribe(service string) (<-chan *LogEntry, func()) {
	s := &subscriber{
		ch:     make(chan *LogEntry, 256),
		cancel: make(chan struct{}),
	}

	a.subMu.Lock()
	a.subscribers[service] = append(a.subscribers[service], s)
	a.subMu.Unlock()

	unsubscribe := func() {
		close(s.cancel)

		a.subMu.Lock()
		defer a.subMu.Unlock()
		subs := a.subscribers[service]
		for i, sub := range subs {
			if sub == s {
				a.subscribers[service] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(s.ch)
	}

	return s.ch, unsubscribe
}

// ExportLoki serialises the given log entries into the Loki HTTP push API
// JSON format (POST /loki/api/v1/push). Entries are grouped by their label
// set into separate streams.
func (a *Aggregator) ExportLoki(entries []*LogEntry) ([]byte, error) {
	type lokiValue [2]string // [timestamp_ns, line]
	type lokiStream struct {
		Stream map[string]string `json:"stream"`
		Values []lokiValue       `json:"values"`
	}
	type lokiPush struct {
		Streams []lokiStream `json:"streams"`
	}

	// Group entries by their label fingerprint.
	streamMap := make(map[string]*lokiStream)

	for _, entry := range entries {
		labels := make(map[string]string)
		labels["service"] = entry.Service
		labels["level"] = string(entry.Level)
		for k, v := range entry.Labels {
			labels[k] = v
		}

		key := labelsKey(labels)
		ls, ok := streamMap[key]
		if !ok {
			ls = &lokiStream{
				Stream: labels,
				Values: make([]lokiValue, 0),
			}
			streamMap[key] = ls
		}

		tsNano := strconv.FormatInt(entry.Timestamp.UnixNano(), 10)
		line := entry.Message
		if len(entry.Fields) > 0 {
			fieldsJSON, _ := json.Marshal(entry.Fields)
			line = fmt.Sprintf("%s %s", entry.Message, string(fieldsJSON))
		}
		ls.Values = append(ls.Values, lokiValue{tsNano, line})
	}

	streams := make([]lokiStream, 0, len(streamMap))
	for _, s := range streamMap {
		streams = append(streams, *s)
	}

	push := lokiPush{Streams: streams}
	data, err := json.Marshal(push)
	if err != nil {
		return nil, fmt.Errorf("marshal loki push: %w", err)
	}
	return data, nil
}

// labelsKey produces a deterministic string key for a label set.
func labelsKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b []byte
	for _, k := range keys {
		b = append(b, k...)
		b = append(b, '=')
		b = append(b, labels[k]...)
		b = append(b, ',')
	}
	return string(b)
}

// Tail returns the most recent n log entries for the given service. If service
// is empty, it returns from the global buffer.
func (a *Aggregator) Tail(service string, lines int) []*LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var source []*LogEntry
	if service != "" {
		source = a.streams[service]
	} else {
		source = a.buffer
	}

	if lines <= 0 || lines > len(source) {
		lines = len(source)
	}

	start := len(source) - lines
	result := make([]*LogEntry, lines)
	copy(result, source[start:])
	return result
}

// Flush clears all buffered log entries and per-service streams.
func (a *Aggregator) Flush() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.buffer = make([]*LogEntry, 0, a.maxBufferSize)
	a.streams = make(map[string][]*LogEntry)
}

// Len returns the total number of entries in the global buffer.
func (a *Aggregator) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.buffer)
}
