package pipeline_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/pipeline"
)

func mkEvent(src, raw string) event.Event {
	return event.Event{Source: src, Raw: raw, Timestamp: time.Now()}
}

func TestMerger_PerSourceOrder(t *testing.T) {
	m := pipeline.New(1024)
	defer m.Close()

	a := make(chan event.Event, 16)
	b := make(chan event.Event, 16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Add(ctx, a)
	m.Add(ctx, b)

	const n = 20
	for i := 0; i < n; i++ {
		a <- mkEvent("a", strconv.Itoa(i))
	}
	close(a)
	for i := 0; i < n; i++ {
		b <- mkEvent("b", strconv.Itoa(i))
	}
	close(b)

	got := map[string][]string{}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for len(got["a"])+len(got["b"]) < 2*n {
		select {
		case ev := <-m.Events():
			got[ev.Source] = append(got[ev.Source], ev.Raw)
		case <-deadline.C:
			t.Fatalf("timed out; got=%v", got)
		}
	}
	for _, src := range []string{"a", "b"} {
		for i, raw := range got[src] {
			if raw != strconv.Itoa(i) {
				t.Fatalf("%s out of order at %d: got %q", src, i, raw)
			}
		}
	}
}

func TestMerger_PauseAndResume(t *testing.T) {
	m := pipeline.New(64)
	defer m.Close()

	in := make(chan event.Event, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Add(ctx, in)

	m.Pause()
	// Push 5 events while paused; nothing should arrive.
	for i := 0; i < 5; i++ {
		in <- mkEvent("s", strconv.Itoa(i))
	}
	select {
	case ev := <-m.Events():
		t.Fatalf("expected no events while paused, got: %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
	// Wait for events to land in the ring buffer.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if m.Stats().Buffered == 5 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if s := m.Stats(); s.Buffered != 5 || !s.Paused {
		t.Fatalf("stats while paused: %+v", s)
	}

	m.Resume()
	for i := 0; i < 5; i++ {
		select {
		case ev := <-m.Events():
			if ev.Raw != strconv.Itoa(i) {
				t.Fatalf("ev %d raw %q", i, ev.Raw)
			}
		case <-time.After(time.Second):
			t.Fatalf("missed event %d after resume", i)
		}
	}
}

func TestMerger_Clear(t *testing.T) {
	m := pipeline.New(64)
	defer m.Close()

	in := make(chan event.Event, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Add(ctx, in)

	m.Pause()
	for i := 0; i < 3; i++ {
		in <- mkEvent("s", strconv.Itoa(i))
	}
	// Wait until they reach the buffer.
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if m.Stats().Buffered == 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.Clear()
	if s := m.Stats(); s.Buffered != 0 {
		t.Fatalf("expected empty after Clear, got: %+v", s)
	}
	m.Resume()
	select {
	case ev := <-m.Events():
		t.Fatalf("expected no events after Clear, got: %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMerger_RingDropsOldest(t *testing.T) {
	m := pipeline.New(4)
	defer m.Close()

	in := make(chan event.Event, 32)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Add(ctx, in)

	m.Pause()
	for i := 0; i < 10; i++ {
		in <- mkEvent("s", strconv.Itoa(i))
	}
	// Wait until producer has fed everything in.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if m.Stats().Dropped == 6 && m.Stats().Buffered == 4 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if s := m.Stats(); s.Buffered != 4 || s.Dropped != 6 {
		t.Fatalf("expected 4 buffered / 6 dropped, got: %+v", s)
	}
	m.Resume()
	// The remaining events should be the four newest: 6,7,8,9.
	for _, want := range []string{"6", "7", "8", "9"} {
		select {
		case ev := <-m.Events():
			if ev.Raw != want {
				t.Fatalf("want %q got %q", want, ev.Raw)
			}
		case <-time.After(time.Second):
			t.Fatalf("missed %q", want)
		}
	}
}

func TestMerger_CloseShutsDownEvenWithBlockedProducer(t *testing.T) {
	m := pipeline.New(2)
	in := make(chan event.Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Add(ctx, in)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Pump events; Close must unblock us within a reasonable time.
		for i := 0; i < 1000; i++ {
			select {
			case in <- mkEvent("s", strconv.Itoa(i)):
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	m.Close()
	// Output channel must close.
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-m.Events():
			if !ok {
				cancel()
				wg.Wait()
				return
			}
		case <-timer.C:
			t.Fatal("Events() did not close after Close()")
		}
	}
}
