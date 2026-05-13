package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loglens/loglens/internal/event"
)

// PumpEvents reads from in and delivers EventBatch messages to prog. Events
// are coalesced into batches of up to PumpMaxBatch and flushed on every
// PumpInterval tick (~60fps). This keeps the Bubble Tea message bus from
// being flooded during log bursts while still feeling real-time at normal
// event rates.
//
// Returns when ctx is cancelled or in is closed. A final flush is performed
// on exit so no buffered events are silently dropped.
func PumpEvents(ctx context.Context, prog Sender, in <-chan event.Event) {
	const (
		PumpInterval = 16 * time.Millisecond
		PumpMaxBatch = 256
	)
	t := time.NewTicker(PumpInterval)
	defer t.Stop()

	var batch []event.Event
	flush := func() {
		if len(batch) == 0 {
			return
		}
		out := make(EventBatch, len(batch))
		copy(out, batch)
		prog.Send(out)
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case ev, ok := <-in:
			if !ok {
				flush()
				return
			}
			batch = append(batch, ev)
			if len(batch) >= PumpMaxBatch {
				flush()
			}
		case <-t.C:
			flush()
		}
	}
}

// Sender is the subset of *tea.Program that PumpEvents needs — stated as an
// interface so tests can drive the pump without a real terminal program.
type Sender interface {
	Send(msg tea.Msg)
}
