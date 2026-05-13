package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loglens/loglens/internal/event"
)

func TestViewContainsBranding(t *testing.T) {
	m := New()
	// Pre-WindowSizeMsg: placeholder must have branding + quit hint.
	out := m.View()
	if !strings.Contains(out, "LogLens") {
		t.Fatalf("expected branding in placeholder view, got: %q", out)
	}
	if !strings.Contains(out, "q to quit") {
		t.Fatalf("expected quit hint in placeholder view, got: %q", out)
	}
	// Post-WindowSizeMsg: multi-pane shell must have branding + quit binding.
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	rendered := sized.View()
	if !strings.Contains(rendered, "LogLens") {
		t.Fatalf("expected branding in multi-pane view, got: %q", rendered)
	}
	if !strings.Contains(stripANSI(rendered), "quit") {
		t.Fatalf("expected quit binding in footer, got: %q", rendered)
	}
}

func TestEventBatch(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	m := New()
	batch := EventBatch{
		{Timestamp: now, Source: "file:///a.log", Raw: "line 1", Level: event.LevelInfo},
		{Timestamp: now, Source: "file:///a.log", Raw: "line 2", Level: event.LevelError},
	}
	next, _ := m.Update(batch)
	got := next.(Model)
	if want := 2; len(got.events) != want {
		t.Fatalf("EventBatch: want %d events, got %d", want, len(got.events))
	}
	if got.VisibleRows()[1].Raw != "line 2" {
		t.Fatalf("second event mismatch: %+v", got.VisibleRows())
	}
}

func TestQuitKey(t *testing.T) {
	m := New()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected tea.Quit command after q key, got nil")
	}
	if next.View() != "" {
		t.Fatalf("expected empty view after quit, got: %q", next.View())
	}
}

func TestWindowResize(t *testing.T) {
	m := New()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(Model)
	if got.width != 120 || got.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", got.width, got.height)
	}
}

// TestSlashFilterIntegration drives the slash-prompt end-to-end: ingest
// events, open the prompt with "/", type a predicate, observe the live row
// count change, then commit with Enter and cancel a second prompt with Esc.
func TestSlashFilterIntegration(t *testing.T) {
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	m := New()
	m = m.Ingest(event.Event{Timestamp: now, Source: "file:///app.log", Raw: "boom", Level: event.LevelError})
	m = m.Ingest(event.Event{Timestamp: now, Source: "file:///app.log", Raw: "ok", Level: event.LevelInfo})
	m = m.Ingest(event.Event{Timestamp: now, Source: "k8s://prod/api-0", Raw: "request handled", Level: event.LevelInfo})

	if got := len(m.VisibleRows()); got != 3 {
		t.Fatalf("baseline: want 3 visible rows, got %d", got)
	}

	// Press "/" to open the filter prompt.
	next, _ := m.Update(slash())
	m = next.(Model)
	if !m.InputActive() {
		t.Fatal("expected slash to open filter input")
	}

	// Type "level=error" one rune at a time, asserting the live preview.
	for _, r := range "level=error" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if got, want := m.InputBuffer(), "level=error"; got != want {
		t.Fatalf("input buffer: got %q want %q", got, want)
	}
	if got := len(m.VisibleRows()); got != 1 {
		t.Fatalf("live preview: want 1 visible row, got %d", got)
	}

	// Commit with Enter.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.InputActive() {
		t.Fatal("expected Enter to close input")
	}
	if got := m.FilterText(); got != "level=error" {
		t.Fatalf("committed filter: got %q", got)
	}
	if got := len(m.VisibleRows()); got != 1 {
		t.Fatalf("after commit: want 1 visible row, got %d", got)
	}

	// Open again and type a different filter, then Esc to cancel.
	next, _ = m.Update(slash())
	m = next.(Model)
	if !m.InputActive() {
		t.Fatal("expected second slash to reopen input")
	}
	// Buffer prefilled with current committed text.
	if got, want := m.InputBuffer(), "level=error"; got != want {
		t.Fatalf("prefill: got %q want %q", got, want)
	}
	// Clear with Ctrl+U, then type a noop substring that hides every row.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(Model)
	for _, r := range "zzznomatch" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if got := len(m.VisibleRows()); got != 0 {
		t.Fatalf("preview should hide all rows, got %d visible", got)
	}
	// Esc cancels — committed filter restored.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.InputActive() {
		t.Fatal("expected Esc to close input")
	}
	if got := m.FilterText(); got != "level=error" {
		t.Fatalf("committed filter after esc: got %q, expected unchanged", got)
	}
	if got := len(m.VisibleRows()); got != 1 {
		t.Fatalf("after esc: want 1 row, got %d", got)
	}
}

func TestNegationAndSubstring(t *testing.T) {
	m := New()
	m = m.Ingest(event.Event{Source: "k8s://prod/api", Raw: "handled ok", Level: event.LevelInfo})
	m = m.Ingest(event.Event{Source: "file:///app.log", Raw: "boom", Level: event.LevelError})

	// Slash open + type "!level=error" → hide the error row.
	next, _ := m.Update(slash())
	m = next.(Model)
	for _, r := range "!level=error" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if got := len(m.VisibleRows()); got != 1 {
		t.Fatalf("!level=error preview should leave 1 row, got %d", got)
	}
	// Bareword substring against Raw.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(Model)
	for _, r := range "boom" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if got := len(m.VisibleRows()); got != 1 {
		t.Fatalf("boom preview should leave 1 row, got %d", got)
	}
}

func slash() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} }

func TestHelpQuitKey(t *testing.T) {
	m := New()
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = m2.(Model)
	// Open help
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = m3.(Model)
	if !m.help.IsOpen() {
		t.Fatal("expected help to be open after ?")
	}
	// Send q while help open
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit command after q in help mode, got nil")
	}
}
