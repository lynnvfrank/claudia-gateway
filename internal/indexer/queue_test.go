package indexer

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

func TestQueue_DedupesAndBounds(t *testing.T) {
	q := NewQueue(2)
	j := Job{Root: Root{ID: "r"}, RelPath: "a"}
	if !q.Enqueue(j) {
		t.Fatal("first enqueue should succeed")
	}
	if !q.Enqueue(j) {
		t.Fatal("dedup enqueue should still return true")
	}
	if q.Len() != 1 {
		t.Fatalf("len=%d, want 1", q.Len())
	}
	if !q.Enqueue(Job{Root: Root{ID: "r"}, RelPath: "b"}) {
		t.Fatal("enqueue b")
	}
	if q.Enqueue(Job{Root: Root{ID: "r"}, RelPath: "c"}) {
		t.Fatal("queue should be full")
	}
}

func TestQueue_DequeueBlocksThenReturns(t *testing.T) {
	q := NewQueue(4)
	got := make(chan Job, 1)
	go func() {
		j, ok := q.Dequeue(context.Background())
		if ok {
			got <- j
		}
	}()
	time.Sleep(10 * time.Millisecond)
	q.Enqueue(Job{Root: Root{ID: "r"}, RelPath: "a"})
	select {
	case j := <-got:
		if j.RelPath != "a" {
			t.Fatalf("got %+v", j)
		}
	case <-time.After(time.Second):
		t.Fatal("dequeue did not return")
	}
}

func TestQueue_DequeueExitsOnContext(t *testing.T) {
	q := NewQueue(4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() {
		_, ok := q.Dequeue(ctx)
		done <- ok
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("expected !ok on cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("dequeue did not unblock on cancel")
	}
}

func TestBackoff_GrowsAndCaps(t *testing.T) {
	base := 10 * time.Millisecond
	max := 80 * time.Millisecond
	d0 := Backoff(0, base, max, nil)
	d1 := Backoff(1, base, max, nil)
	d3 := Backoff(3, base, max, nil)
	d10 := Backoff(10, base, max, nil)
	if d0 != base || d1 != 2*base || d3 != 8*base || d10 != max {
		t.Fatalf("d0=%v d1=%v d3=%v d10=%v", d0, d1, d3, d10)
	}
	rng := rand.New(rand.NewSource(1))
	if got := Backoff(2, base, max, rng); got > 4*base {
		t.Fatalf("jittered backoff exceeded ceiling: %v", got)
	}
}
