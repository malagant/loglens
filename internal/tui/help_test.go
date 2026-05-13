package tui

import (
	"strings"
	"testing"
)

func TestHelpOpenClose(t *testing.T) {
	h := newHelp(DefaultKeymap)
	if h.IsOpen() {
		t.Fatal("help should start closed")
	}
	h.Open()
	if !h.IsOpen() {
		t.Fatal("should be open after Open()")
	}
	h.Close()
	if h.IsOpen() {
		t.Fatal("should be closed after Close()")
	}
	h.Toggle()
	if !h.IsOpen() {
		t.Fatal("Toggle from closed should open")
	}
	h.Toggle()
	if h.IsOpen() {
		t.Fatal("Toggle from open should close")
	}
}

func TestHelpViewWhenClosed(t *testing.T) {
	h := newHelp(DefaultKeymap)
	if got := h.View(80, 24); got != "" {
		t.Fatalf("closed help should be empty, got %q", got)
	}
}

func TestHelpViewListsAllBindings(t *testing.T) {
	h := newHelp(DefaultKeymap)
	h.Open()
	plain := stripANSI(h.View(120, 40))
	for _, b := range DefaultKeymap.All() {
		if !strings.Contains(plain, b.Help) {
			t.Errorf("help missing label %q", b.Help)
		}
	}
	if !strings.Contains(plain, "Basic") {
		t.Error("missing Basic section header")
	}
	if !strings.Contains(plain, "Power user") {
		t.Error("missing Power user section header")
	}
}

func TestHelpScrollClamps(t *testing.T) {
	h := newHelp(DefaultKeymap)
	h.Open()
	h.ScrollUp()
	if h.scroll != 0 {
		t.Fatalf("ScrollUp from 0 should clamp, got %d", h.scroll)
	}
	for i := 0; i < 1000; i++ {
		h.ScrollDown(3)
	}
	max := h.lineCount() - 3
	if max < 0 {
		max = 0
	}
	if h.scroll != max {
		t.Fatalf("ScrollDown should clamp at %d, got %d", max, h.scroll)
	}
}

func TestHelpScrollChangesView(t *testing.T) {
	h := newHelp(DefaultKeymap)
	h.Open()
	before := h.View(60, 10)
	h.ScrollDown(4)
	after := h.View(60, 10)
	if before == after {
		t.Fatal("scroll should change visible body")
	}
}

// stripANSI removes ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				j++
				if c >= 0x40 && c <= 0x7e {
					break
				}
			}
			i = j - 1
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
