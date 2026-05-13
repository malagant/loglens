// Package tui contains the LogLens terminal UI model. The current build is
// still a scaffold — SPA-22 will replace it with the real multi-pane layout.
// What lives here today is the minimum to drive the slash-prompt filter from
// SPA-23: an event-row list, a filter input, and a committed query.
package tui

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/filter"
)

// Version is overridden at build time by GoReleaser via -ldflags.
var Version = "dev"

// defaultMaxRows caps the in-memory event ring used by the scaffold model.
// SPA-22 will replace this with the pipeline-driven viewport.
const defaultMaxRows = 5000

// EventMsg is delivered by the pipeline to push a new event onto the model.
type EventMsg event.Event

// Model is the root Bubble Tea model.
type Model struct {
	width, height int
	quit          bool

	events  []event.Event
	maxRows int

	committed     filter.Query
	committedText string

	inputMode  bool
	inputBuf   []rune
	preview    filter.Query
	previewErr error
}

// New returns a fresh Model with default state.
func New() Model {
	return Model{maxRows: defaultMaxRows}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Ingest appends an event to the in-memory row buffer. It is exported for
// tests and for the pipeline glue that will land with SPA-22.
func (m Model) Ingest(ev event.Event) Model {
	if m.maxRows == 0 {
		m.maxRows = defaultMaxRows
	}
	m.events = append(m.events, ev)
	if len(m.events) > m.maxRows {
		m.events = m.events[len(m.events)-m.maxRows:]
	}
	return m
}

// FilterText returns the currently committed filter text (empty when no
// filter is active). Useful for tests.
func (m Model) FilterText() string { return m.committedText }

// InputActive reports whether the slash-prompt is open.
func (m Model) InputActive() bool { return m.inputMode }

// InputBuffer returns the in-progress filter text while the prompt is open.
func (m Model) InputBuffer() string { return string(m.inputBuf) }

// VisibleRows returns the events that pass the currently active query
// (preview while typing, otherwise the committed one). It is exported so the
// integration test can assert on row count changes.
func (m Model) VisibleRows() []event.Event {
	q := m.activeQuery()
	if q.Empty() {
		return m.events
	}
	out := m.events[:0:0]
	for _, ev := range m.events {
		if q.Match(ev) {
			out = append(out, ev)
		}
	}
	return out
}

func (m Model) activeQuery() filter.Query {
	if m.inputMode {
		return m.preview
	}
	return m.committed
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case EventMsg:
		return m.Ingest(event.Event(msg)), nil

	case tea.KeyMsg:
		if m.inputMode {
			return m.updateInput(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.quit = true
		return m, tea.Quit
	case "/":
		m.inputMode = true
		m.inputBuf = append(m.inputBuf[:0], []rune(m.committedText)...)
		m.preview = m.committed
		m.previewErr = nil
		return m, nil
	}
	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.inputMode = false
		m.inputBuf = m.inputBuf[:0]
		m.preview = filter.Query{}
		m.previewErr = nil
		return m, nil
	case tea.KeyEnter:
		m.committed = m.preview
		m.committedText = string(m.inputBuf)
		m.inputMode = false
		m.inputBuf = m.inputBuf[:0]
		m.preview = filter.Query{}
		m.previewErr = nil
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		if n := len(m.inputBuf); n > 0 {
			m.inputBuf = m.inputBuf[:n-1]
		}
	case tea.KeyCtrlU:
		m.inputBuf = m.inputBuf[:0]
	case tea.KeyRunes, tea.KeySpace:
		for _, r := range msg.Runes {
			if r == 0 || unicode.IsControl(r) {
				continue
			}
			m.inputBuf = append(m.inputBuf, r)
		}
	default:
		// Pass through; nothing to do for unknown keys in input mode.
		return m, nil
	}
	m.preview, m.previewErr = filter.Parse(string(m.inputBuf))
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

	promptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	matchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	dimStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("#555555"))
)

// View implements tea.Model.
func (m Model) View() string {
	if m.quit {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n  ")
	b.WriteString(titleStyle.Render("LogLens"))
	b.WriteString("\n  ")
	b.WriteString(bodyStyle.Render("one TUI, every log source, one query language"))
	b.WriteString("\n\n")

	rows := m.VisibleRows()
	if len(m.events) == 0 {
		b.WriteString("  ")
		b.WriteString(dimStyle.Render("waiting for events…"))
		b.WriteString("\n")
	} else {
		for _, ev := range rows {
			b.WriteString("  ")
			b.WriteString(matchStyle.Render(renderRow(ev)))
			b.WriteString("\n")
		}
		if len(rows) == 0 {
			b.WriteString("  ")
			b.WriteString(dimStyle.Render("no rows match current filter"))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n  ")
	b.WriteString(m.footer(len(rows)))
	b.WriteString("\n")
	return b.String()
}

func (m Model) footer(visible int) string {
	if m.inputMode {
		cursor := "│"
		return promptStyle.Render("/"+string(m.inputBuf)) + cursor + "  " +
			hintStyle.Render(fmt.Sprintf("enter to commit · esc to cancel · %d/%d rows", visible, len(m.events)))
	}
	filterStatus := "no filter"
	if !m.committed.Empty() {
		filterStatus = "filter: " + m.committedText
	}
	return hintStyle.Render(fmt.Sprintf(
		"version %s — %s · %d/%d rows · / to filter · q to quit",
		Version, filterStatus, visible, len(m.events),
	))
}

func renderRow(ev event.Event) string {
	ts := ev.Timestamp.Format("15:04:05.000")
	lvl := string(ev.Level)
	if lvl == "" {
		lvl = "-"
	}
	src := ev.Source
	if src == "" {
		src = "-"
	}
	return fmt.Sprintf("%s  %-5s  %-24s  %s", ts, lvl, truncate(src, 24), ev.Raw)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
