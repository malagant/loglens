package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFooterFitsEightyCols(t *testing.T) {
	out := renderFooter(80, DefaultKeymap, false)
	if w := lipgloss.Width(out); w != 80 {
		t.Fatalf("footer width = %d, want 80", w)
	}
	plain := stripANSI(out)
	if strings.Contains(plain, "\n") {
		t.Fatal("footer must be a single line")
	}
	for _, must := range []string{"k", "/", "?", "q"} {
		if !strings.Contains(plain, must) {
			t.Errorf("footer missing %q in 80-col output: %q", must, plain)
		}
	}
}

func TestFooterNarrowTruncates(t *testing.T) {
	out := renderFooter(40, DefaultKeymap, false)
	if w := lipgloss.Width(out); w != 40 {
		t.Fatalf("footer width = %d, want 40", w)
	}
	plain := stripANSI(out)
	// First binding must always appear.
	if !strings.Contains(plain, "k") {
		t.Errorf("narrow footer dropped first binding: %q", plain)
	}
}

func TestFooterPausedLabel(t *testing.T) {
	paused := stripANSI(renderFooter(120, DefaultKeymap, true))
	running := stripANSI(renderFooter(120, DefaultKeymap, false))
	if !strings.Contains(paused, "resume") {
		t.Errorf("paused footer should show 'resume': %q", paused)
	}
	if strings.Contains(running, "resume") {
		t.Errorf("running footer should not show 'resume': %q", running)
	}
	if !strings.Contains(running, "pause") {
		t.Errorf("running footer should show 'pause': %q", running)
	}
}

func TestFooterZeroWidth(t *testing.T) {
	if got := renderFooter(0, DefaultKeymap, false); got != "" {
		t.Fatalf("zero-width footer should be empty, got %q", got)
	}
}
