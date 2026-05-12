package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewContainsBranding(t *testing.T) {
	m := New()
	out := m.View()
	if !strings.Contains(out, "LogLens") {
		t.Fatalf("expected branding in view, got: %q", out)
	}
	if !strings.Contains(out, "press q to quit") {
		t.Fatalf("expected quit hint, got: %q", out)
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
