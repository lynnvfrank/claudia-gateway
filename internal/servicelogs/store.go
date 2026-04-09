// Package servicelogs captures line-oriented process and gateway output for the operator UI.
package servicelogs

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMaxLines is the default ring buffer capacity (oldest dropped).
const DefaultMaxLines = 10000

// Entry is one logical log line with a stable sequence number for polling.
type Entry struct {
	Seq    uint64    `json:"seq"`
	Source string    `json:"source"`
	Text   string    `json:"text"`
	Time   time.Time `json:"ts"`
}

// Store is a bounded, thread-safe log buffer with optional SSE subscribers.
type Store struct {
	mu       sync.Mutex
	maxLines int
	lines    []Entry
	lastSeq  uint64

	subsMu sync.Mutex
	subs   map[uint64]chan Entry
	subID  uint64
}

// New creates a Store that retains at most maxLines entries.
func New(maxLines int) *Store {
	if maxLines < 1 {
		maxLines = DefaultMaxLines
	}
	return &Store{
		maxLines: maxLines,
		subs:     make(map[uint64]chan Entry),
	}
}

// Writer returns an io.Writer that splits on '\n' and records complete lines under source.
// Each Writer keeps its own partial-line buffer (safe for separate stdout/stderr pipes).
func (s *Store) Writer(source string) io.Writer {
	return &lineWriter{store: s, source: source}
}

func (s *Store) add(source, text string) {
	text = string(bytes.TrimSuffix([]byte(text), []byte{'\r'}))
	if text == "" {
		return
	}
	now := time.Now().UTC()
	seq := atomic.AddUint64(&s.lastSeq, 1)
	ent := Entry{Seq: seq, Source: source, Text: text, Time: now}

	s.mu.Lock()
	s.lines = append(s.lines, ent)
	if len(s.lines) > s.maxLines {
		overflow := len(s.lines) - s.maxLines
		s.lines = append([]Entry(nil), s.lines[overflow:]...)
	}
	s.mu.Unlock()

	s.broadcast(ent)
}

func (s *Store) broadcast(ent Entry) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for _, ch := range s.subs {
		select {
		case ch <- ent:
		default:
			// Slow consumer: drop — never block the logging path.
		}
	}
}

// Snapshot returns a copy of all buffered lines (oldest first).
func (s *Store) Snapshot() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.lines))
	copy(out, s.lines)
	return out
}

// EntriesAfter returns entries with Seq > afterSeq, and the highest Seq in the buffer (or afterSeq if empty).
func (s *Store) EntriesAfter(afterSeq uint64) (entries []Entry, maxSeq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	maxSeq = afterSeq
	for _, e := range s.lines {
		if e.Seq > maxSeq {
			maxSeq = e.Seq
		}
		if e.Seq > afterSeq {
			entries = append(entries, e)
		}
	}
	return entries, maxSeq
}

// Tail returns the last n entries (n <= 0 means all).
func (s *Store) Tail(n int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 || n >= len(s.lines) {
		out := make([]Entry, len(s.lines))
		copy(out, s.lines)
		return out
	}
	return append([]Entry(nil), s.lines[len(s.lines)-n:]...)
}

// Subscribe registers a consumer for new entries after this call. Buffer is the channel capacity
// (small values drop slow readers). The caller must call cancel when done, which closes the channel.
func (s *Store) Subscribe(buf int) (ch <-chan Entry, cancel func()) {
	if buf < 1 {
		buf = 16
	}
	c := make(chan Entry, buf)
	id := atomic.AddUint64(&s.subID, 1)

	s.subsMu.Lock()
	s.subs[id] = c
	s.subsMu.Unlock()

	return c, func() {
		s.subsMu.Lock()
		if ch2, ok := s.subs[id]; ok {
			delete(s.subs, id)
			close(ch2)
		}
		s.subsMu.Unlock()
	}
}

type lineWriter struct {
	store  *Store
	source string
	buf    []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.store.add(w.source, line)
		w.buf = append([]byte(nil), w.buf[i+1:]...)
	}
	// Avoid unbounded growth if a source never sends '\n'.
	const maxFrag = 64 << 10
	if len(w.buf) > maxFrag {
		w.store.add(w.source, string(w.buf))
		w.buf = w.buf[:0]
	}
	return len(p), nil
}
