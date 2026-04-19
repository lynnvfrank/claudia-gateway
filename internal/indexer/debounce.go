package indexer

import (
	"sync"
	"time"
)

// debouncer collapses a burst of events on the same key into one delayed
// callback. It is safe for concurrent use.
type debouncer struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	delay  time.Duration
	fn     func(string)
	closed bool
}

func newDebouncer(delay time.Duration, fn func(string)) *debouncer {
	return &debouncer{timers: map[string]*time.Timer{}, delay: delay, fn: fn}
}

// Trigger schedules fn(key) after the configured delay. If Trigger is called
// again with the same key before fn fires, the timer is reset.
func (d *debouncer) Trigger(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if t, ok := d.timers[key]; ok {
		t.Stop()
	}
	d.timers[key] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
		d.fn(key)
	})
}

// Close stops every pending timer and prevents future triggers.
func (d *debouncer) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	for _, t := range d.timers {
		t.Stop()
	}
	d.timers = nil
}
