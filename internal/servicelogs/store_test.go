package servicelogs

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLineWriter_splitsLinesAndCarriageReturn(t *testing.T) {
	s := New(100)
	w := s.Writer("test")
	_, _ = io.WriteString(w, "hello\r\nworld\n")
	lines := s.Snapshot()
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %#v", len(lines), lines)
	}
	if lines[0].Source != "test" || lines[0].Text != "hello" {
		t.Fatalf("line0: %+v", lines[0])
	}
	if lines[1].Text != "world" {
		t.Fatalf("line1: %+v", lines[1])
	}
}

func TestLineWriter_splitAcrossWrites(t *testing.T) {
	s := New(100)
	w := s.Writer("a")
	_, _ = w.Write([]byte("part1"))
	_, _ = w.Write([]byte("end\nnext\n"))
	got := s.Snapshot()
	if len(got) != 2 || got[0].Text != "part1end" || got[1].Text != "next" {
		t.Fatalf("got %+v", got)
	}
}

func TestStore_maxLinesEvictsOldest(t *testing.T) {
	s := New(3)
	for i := range 5 {
		s.add("g", fmt.Sprintf("L%d", i))
	}
	got := s.Snapshot()
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Text != "L2" || got[2].Text != "L4" {
		t.Fatalf("expected L2,L3,L4 got %#v", got)
	}
}

func TestEntriesAfter_cursor(t *testing.T) {
	s := New(100)
	s.add("x", "a")
	s.add("x", "b")
	ent, maxSeq := s.EntriesAfter(0)
	if len(ent) != 2 || maxSeq != 2 {
		t.Fatalf("after 0: n=%d max=%d", len(ent), maxSeq)
	}
	ent2, _ := s.EntriesAfter(1)
	if len(ent2) != 1 || ent2[0].Text != "b" {
		t.Fatalf("after 1: %+v", ent2)
	}
}

func TestSubscribe_deliversNewEntries(t *testing.T) {
	s := New(100)
	ch, cancel := s.Subscribe(8)
	defer cancel()

	s.add("gw", "one")
	select {
	case e := <-ch:
		if e.Text != "one" || e.Source != "gw" {
			t.Fatalf("got %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}

func TestSubscribe_slowConsumerDoesNotBlockAdd(t *testing.T) {
	s := New(100)
	_, cancel := s.Subscribe(1)
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			s.add("x", "line")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("add blocked on slow subscriber")
	}
}

func TestConcurrentWriters_noPanic(t *testing.T) {
	s := New(500)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w := s.Writer(fmt.Sprintf("w%d", id))
			for j := 0; j < 50; j++ {
				_, _ = io.WriteString(w, fmt.Sprintf("line-%d-%d\n", id, j))
			}
		}(i)
	}
	wg.Wait()
	if n := len(s.Snapshot()); n != 400 {
		t.Fatalf("expected 400 lines, got %d", n)
	}
}

func TestWriter_largeLineWithoutNewline_flushedWhenOverCap(t *testing.T) {
	s := New(100)
	w := s.Writer("big")
	huge := strings.Repeat("x", 70<<10)
	_, err := io.WriteString(w, huge)
	if err != nil {
		t.Fatal(err)
	}
	lines := s.Snapshot()
	if len(lines) < 1 {
		t.Fatal("expected flush of oversized fragment")
	}
}
