package indexer

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// Job is a single ingest unit handed to workers.
type Job struct {
	Root    Root
	RelPath string
	AbsPath string
}

// Key returns the deduplication key used by the in-memory queue: root id +
// relative path is unique per workspace.
func (j Job) Key() string { return j.Root.ID + "\x00" + j.RelPath }

// Queue is a bounded FIFO with set-style deduplication so rapid filesystem
// events for the same path collapse into a single pending job.
type Queue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	items   []Job
	pending map[string]struct{}
	cap     int
	closed  bool
}

// NewQueue creates a queue with the given capacity. Capacity <= 0 means
// unbounded (still recommended to set a value).
func NewQueue(capacity int) *Queue {
	q := &Queue{cap: capacity, pending: map[string]struct{}{}}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a job. If a job with the same Key is already pending, it is
// dropped (event coalescing). Returns false if the queue is full.
func (q *Queue) Enqueue(j Job) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	if _, ok := q.pending[j.Key()]; ok {
		return true
	}
	if q.cap > 0 && len(q.items) >= q.cap {
		return false
	}
	q.items = append(q.items, j)
	q.pending[j.Key()] = struct{}{}
	q.cond.Signal()
	return true
}

// Dequeue blocks until a job is available or the queue is closed.
func (q *Queue) Dequeue(ctx context.Context) (Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		// Wake the cond when the context is cancelled so the goroutine exits.
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				q.cond.Broadcast()
			case <-done:
			}
		}()
		q.cond.Wait()
		close(done)
		if ctx.Err() != nil {
			return Job{}, false
		}
	}
	if len(q.items) == 0 {
		return Job{}, false
	}
	j := q.items[0]
	q.items = q.items[1:]
	delete(q.pending, j.Key())
	return j, true
}

// Len returns the current number of queued items.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Cap returns the configured capacity (0 means unbounded).
func (q *Queue) Cap() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.cap
}

// Close wakes every blocked Dequeue caller; subsequent Enqueue calls fail.
func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

// Backoff computes the nth retry delay as base * 2^attempt, capped at max,
// with full-jitter randomization. attempt is 0-based.
func Backoff(attempt int, base, max time.Duration, rng *rand.Rand) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := base
	for i := 0; i < attempt && d < max; i++ {
		d *= 2
	}
	if d > max {
		d = max
	}
	if rng != nil {
		d = time.Duration(rng.Int63n(int64(d) + 1))
	}
	return d
}

// ErrPaused is returned by RunWithBackoff when the queue worker stops
// retrying and signals that the supervisor should pause and poll health.
var ErrPaused = errors.New("indexer: paused after exhausted retries")
