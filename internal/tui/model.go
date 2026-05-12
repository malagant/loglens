// Package tui contains the LogLens terminal UI model. The current build is a
// hello-world scaffold; it exists so the foundation work (CI, releases,
// distribution) can be validated end-to-end before the real log views land.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Version is overridden at build time by GoReleaser via -ldflags.
var Version = "dev"

// Model is the root Bubble Tea model.
type Model struct {
	width  int
	height int
	quit   bool
}

// New returns a fresh Model with default state.
func New() Model {
	return Model{}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	hintStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("#888888"))
)

// View implements tea.Model.
func (m Model) View() string {
	if m.quit {
		return ""
	}
	header := titleStyle.Render("LogLens")
	body := bodyStyle.Render("one TUI, every log source, one query language")
	hint := hintStyle.Render(fmt.Sprintf("version %s — press q to quit", Version))
	return fmt.Sprintf("\n  %s\n  %s\n\n  %s\n", header, body, hint)
}
