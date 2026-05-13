package tui

import (
	"testing"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpCloseAndQuit(t *testing.T) {
	m := New()
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = m2.(Model)
	
	// Open help with ?
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = m3.(Model)
	if !m.help.IsOpen() {
		t.Fatal("help should be open")
	}
	
	// Close with ?
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = m4.(Model)
	if m.help.IsOpen() {
		t.Fatal("help should be closed after second ?")
	}
	
	// Now q should quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit after q in normal mode")
	}
}
