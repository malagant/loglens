package tui

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loglens/loglens/internal/event"
)

type recSender struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (r *recSender) Send(msg tea.Msg) {
	r.mu.Lock()
	r.msgs = append(r.msgs, msg)
	r.mu.Unlock()
}

func (r *recSender) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, m := range r.msgs {
		if b, ok := m.(EventBatch); ok {
			n += len(b)
		}
	}
	return n
}

func (r *recSender) batches() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.msgs)
}

func TestPumpCoalescesAndFlushesOnClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan event.Event)
	s := &recSender{}
	done := make(chan struct{})
	go func() {
		PumpEvents(ctx, s, in)
		close(done)
	}()

	const n = 50
	for i := 0; i < n; i++ {
		in <- event.Event{Raw: "row"}
	}
	close(in)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pump did not return after channel close")
	}
	if got := s.total(); got != n {
		t.Fatalf("total events = %d, want %d", got, n)
	}
	if b := s.batches(); b < 1 || b > n {
		t.Fatalf("expected 1..%d batches, got %d", n, b)
	}
}

func TestPumpRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan event.Event)
	s := &recSender{}
	done := make(chan struct{})
	go func() {
		PumpEvents(ctx, s, in)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pump did not return after context cancel")
	}
}
