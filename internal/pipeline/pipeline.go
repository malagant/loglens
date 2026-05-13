// Package pipeline merges events from N concurrent sources into a single
// ordered (per-source) stream backed by a bounded ring buffer. It supports
// pause/clear semantics and drops oldest events when the buffer is full, so
// slow consumers cannot deadlock fast producers.
//
// The buffer is intentionally lossy at the head: when it fills up, older
// events are evicted to make room for new ones. The Stats counter exposes the
// drop count so the UI can surface a "lost events" indicator.
package pipeline

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/loglens/loglens/internal/event"
)

// Merger fans events from any number of source channels into a single output
// channel. It is safe for concurrent calls on every public method.
type Merger struct {
	cap int

	in   chan event.Event
	out  chan event.Event
	done chan struct{}

	pauseCh chan bool
	clearCh chan struct{}
	statsCh chan chan Stats

	dropped atomic.Int64

	closeOnce sync.Once
}

// Stats is a snapshot of the merger's internal state.
type Stats struct {
	Buffered int
	Capacity int
	Dropped  int64
	Paused   bool
}

// New constructs a Merger with the given ring-buffer capacity. capacity must
// be >= 1. The internal loop starts immediately.
func New(capacity int) *Merger {
	if capacity < 1 {
		capacity = 1
	}
	m := &Merger{
		cap:     capacity,
		in:      make(chan event.Event, 64),
		out:     make(chan event.Event),
		done:    make(chan struct{}),
		pauseCh: make(chan bool),
		clearCh: make(chan struct{}),
		statsCh: make(chan chan Stats),
	}
	go m.loop()
	return m
}

// Add wires a source channel into the merger. It returns immediately; a
// goroutine forwards events from ch into the merger until ctx is cancelled or
// ch is closed. Calling Add after Close panics.
func (m *Merger) Add(ctx context.Context, ch <-chan event.Event) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.done:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				select {
				case m.in <- ev:
				case <-ctx.Done():
					return
				case <-m.done:
					return
				}
			}
		}
	}()
}

// Events returns the merged output channel. It is closed when Close is called
// or the loop exits.
func (m *Merger) Events() <-chan event.Event { return m.out }

// Pause stops emitting events to the output channel. Sources continue to feed
// the ring buffer while paused; if the buffer fills, oldest events are dropped.
func (m *Merger) Pause() { m.setPaused(true) }

// Resume re-enables event emission.
func (m *Merger) Resume() { m.setPaused(false) }

func (m *Merger) setPaused(p bool) {
	select {
	case m.pauseCh <- p:
	case <-m.done:
	}
}

// Clear discards all buffered events. The dropped counter is not reset.
func (m *Merger) Clear() {
	select {
	case m.clearCh <- struct{}{}:
	case <-m.done:
	}
}

// Stats returns a snapshot of the merger's state.
func (m *Merger) Stats() Stats {
	reply := make(chan Stats, 1)
	select {
	case m.statsCh <- reply:
		return <-reply
	case <-m.done:
		return Stats{Capacity: m.cap, Dropped: m.dropped.Load()}
	}
}

// Close shuts down the loop and closes the output channel. It is idempotent.
func (m *Merger) Close() {
	m.closeOnce.Do(func() { close(m.done) })
}

func (m *Merger) loop() {
	defer close(m.out)
	buf := make([]event.Event, 0, m.cap)
	paused := false

	for {
		var (
			outCh chan event.Event
			head  event.Event
		)
		if !paused && len(buf) > 0 {
			outCh = m.out
			head = buf[0]
		}

		select {
		case <-m.done:
			return
		case ev := <-m.in:
			if len(buf) == m.cap {
				buf = buf[1:]
				m.dropped.Add(1)
			}
			buf = append(buf, ev)
		case outCh <- head:
			buf = buf[1:]
		case p := <-m.pauseCh:
			paused = p
		case <-m.clearCh:
			buf = buf[:0]
		case reply := <-m.statsCh:
			reply <- Stats{
				Buffered: len(buf),
				Capacity: m.cap,
				Dropped:  m.dropped.Load(),
				Paused:   paused,
			}
		}
	}
}
