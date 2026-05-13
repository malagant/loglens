// Package tui contains the LogLens terminal UI. The Model hosts the
// three-pane shell (sources / stream / detail), the `?` help overlay, the
// keymap registry, the throttled pipeline receiver (EventBatch), and the
// slash-prompt filter from internal/filter.
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loglens/loglens/internal/correlate"
	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/filter"
	"github.com/loglens/loglens/internal/jsonview"
	"github.com/loglens/loglens/internal/tui/style"
)

// Version is overridden at build time by GoReleaser via -ldflags.
var Version = "dev"

// defaultMaxRows caps the in-memory event ring. The pipeline merger has its
// own ring; this cap is purely a UI concern (memory + render latency).
const defaultMaxRows = 5000

// EventMsg pushes a single event to the model. Kept for simple pipelines and
// tests written before EventBatch landed.
type EventMsg event.Event

// EventBatch delivers a slice of events in one Bubble Tea message, coalesced
// by PumpEvents on a ~16ms ticker so a fast source cannot flood the message bus.
type EventBatch []event.Event

// PausedMsg reflects an external pause/resume state change.
type PausedMsg bool

// Pane identifies a focusable region. Tab cycles through them in order.
type Pane int

const (
	PaneSources Pane = iota
	PaneStream
	PaneDetail
)

// Action is emitted to the host when a key binding has a side effect that
// the model cannot handle alone (pipeline pause/resume, clear buffer).
type Action int

const (
	ActionNone Action = iota
	ActionPause
	ActionResume
	ActionClear
)

type sourceEntry struct {
	name  string
	count int
}

// Model is the root tea.Model for the LogLens TUI.
type Model struct {
	width, height int
	quit          bool

	// keymap + overlays
	keys Keymap
	help helpModel

	// pane focus and visibility
	focused       Pane
	detailVisible bool

	// event buffer and selection
	events  []event.Event
	maxRows int
	cursor  int  // index into events; -1 when empty
	follow  bool // follow-tail mode

	// source list derived from observed events (sorted, with counts)
	sources   []sourceEntry
	sourceIdx map[string]int

	// pipeline state
	paused bool

	// slash-prompt filter (preserved from SPA-23)
	committed     filter.Query
	committedText string
	inputMode     bool
	inputBuf      []rune
	preview       filter.Query
	previewErr    error

	// detail pane — populated on enter, holds a jsonview tree
	detail     jsonview.Model
	detailOpen bool

	// request-id correlator — marks rows that share the selected event's id
	detector correlate.Detector

	// host callback for pipeline side effects; nil-safe
	onAction func(Action)
}

// New returns a fresh Model with default state.
func New() Model {
	km := DefaultKeymap
	return Model{
		keys:      km,
		help:      newHelp(km),
		focused:   PaneStream,
		follow:    true,
		maxRows:   defaultMaxRows,
		cursor:    -1,
		sourceIdx: map[string]int{},
		detector:  correlate.Default(),
	}
}

// WithDetector returns a copy of the Model with a custom request-id detector.
// Used by cmd/loglens to plumb the --correlate flag into the running model.
func (m Model) WithDetector(d correlate.Detector) Model {
	if len(d.Keys()) > 0 {
		m.detector = d
	}
	return m
}

// WithActionHandler returns a copy of the Model with an external callback
// installed. Used by cmd/loglens to relay pause/clear to the pipeline merger.
func (m Model) WithActionHandler(fn func(Action)) Model {
	m.onAction = fn
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Ingest appends a single event to the in-memory ring. Exported for tests and
// for single-event producers that haven't moved to EventBatch yet.
func (m Model) Ingest(ev event.Event) Model {
	if m.maxRows == 0 {
		m.maxRows = defaultMaxRows
	}
	if m.sourceIdx == nil {
		m.sourceIdx = map[string]int{}
	}
	m.events = append(m.events, ev)
	if over := len(m.events) - m.maxRows; over > 0 {
		m.events = m.events[over:]
		if m.cursor >= 0 {
			m.cursor -= over
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
	}
	m.bumpSource(ev.Source)
	if m.follow {
		m.cursor = len(m.events) - 1
	} else if m.cursor < 0 && len(m.events) > 0 {
		m.cursor = 0
	}
	return m
}

func (m *Model) bumpSource(name string) {
	if name == "" {
		return
	}
	if i, ok := m.sourceIdx[name]; ok {
		m.sources[i].count++
		return
	}
	m.sources = append(m.sources, sourceEntry{name: name, count: 1})
	sort.SliceStable(m.sources, func(i, j int) bool {
		return m.sources[i].name < m.sources[j].name
	})
	for i, s := range m.sources {
		m.sourceIdx[s.name] = i
	}
}

// FilterText returns the committed filter text (empty when no filter active).
func (m Model) FilterText() string { return m.committedText }

// InputActive reports whether the slash-prompt is open.
func (m Model) InputActive() bool { return m.inputMode }

// InputBuffer returns the in-progress filter text while the prompt is open.
func (m Model) InputBuffer() string { return string(m.inputBuf) }

// VisibleRows returns the events that pass the currently active query
// (preview while typing, otherwise the committed one).
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

	case EventBatch:
		for _, ev := range msg {
			m = m.Ingest(ev)
		}
		return m, nil

	case PausedMsg:
		m.paused = bool(msg)
		return m, nil

	case jsonview.ClipboardWriteMsg:
		return m, osc52Cmd(msg.Content)

	case tea.KeyMsg:
		if m.help.IsOpen() {
			return m.handleHelpKey(msg)
		}
		if m.inputMode {
			return m.updateInput(msg)
		}
		if m.focused == PaneDetail && m.detailOpen {
			return m.updateDetail(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch {
	case m.keys.Help.Matches(k), k == "esc":
		m.help.Close()
	case m.keys.Down.Matches(k):
		m.help.ScrollDown(m.helpViewportH())
	case m.keys.Up.Matches(k):
		m.help.ScrollUp()
	case m.keys.Quit.Matches(k):
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Slash opens the filter prompt (preserved from SPA-23).
	if k == "/" {
		m.inputMode = true
		m.inputBuf = append(m.inputBuf[:0], []rune(m.committedText)...)
		m.preview = m.committed
		m.previewErr = nil
		return m, nil
	}

	switch {
	case m.keys.Quit.Matches(k):
		m.quit = true
		return m, tea.Quit

	case k == "esc":
		if m.detailVisible && m.detailOpen {
			m.detailVisible = false
			m.detailOpen = false
			if m.focused == PaneDetail {
				m.focused = PaneStream
			}
			return m, nil
		}
		m.quit = true
		return m, tea.Quit

	case m.keys.Help.Matches(k):
		m.help.Open()

	case m.keys.CyclePane.Matches(k):
		m.focused = nextPane(m.focused, m.detailVisible && m.detailOpen)

	case m.keys.Detail.Matches(k):
		m.openDetail()

	case m.keys.Pause.Matches(k):
		m.paused = !m.paused
		m.triggerAction()

	case m.keys.Clear.Matches(k):
		m.clearBuffer()
		if m.onAction != nil {
			m.onAction(ActionClear)
		}

	case m.keys.Up.Matches(k):
		m.moveCursor(-1)
	case m.keys.Down.Matches(k):
		m.moveCursor(1)
	case m.keys.JumpTop.Matches(k):
		m.cursor = 0
		m.follow = false
	case m.keys.JumpBot.Matches(k):
		if len(m.events) > 0 {
			m.cursor = len(m.events) - 1
		}
		m.follow = true
	case m.keys.PageDown.Matches(k):
		m.moveCursor(m.streamViewportH())
	case m.keys.PageUp.Matches(k):
		m.moveCursor(-m.streamViewportH())
	case m.keys.Follow.Matches(k):
		m.follow = !m.follow
		if m.follow && len(m.events) > 0 {
			m.cursor = len(m.events) - 1
		}
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
		return m, nil
	}
	m.preview, m.previewErr = filter.Parse(string(m.inputBuf))
	return m, nil
}

// updateDetail forwards key events to the jsonview tree in the detail pane.
func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.detailVisible = false
		m.detailOpen = false
		m.focused = PaneStream
		return m, nil
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

// openDetail populates the detail pane with a jsonview tree for the current event.
func (m *Model) openDetail() {
	if len(m.events) == 0 {
		return
	}
	idx := m.cursor
	if idx < 0 || idx >= len(m.events) {
		idx = 0
	}
	ev := m.events[idx]
	if ev.Fields != nil {
		m.detail = jsonview.New(ev.Fields, ev.Raw)
	} else {
		m.detail = jsonview.NewRaw(ev.Raw)
	}
	m.detailOpen = true
	m.detailVisible = true
}

// selectedRequestID returns the correlate id of the currently selected event,
// or "" when none is found or the detector has nothing to match.
func (m Model) selectedRequestID() string {
	if m.cursor < 0 || m.cursor >= len(m.events) {
		return ""
	}
	return m.detector.ID(m.events[m.cursor])
}

// osc52Cmd emits an OSC 52 clipboard-write escape to stdout from inside a
// tea.Cmd, which runs outside Bubble Tea's render lock.
func osc52Cmd(content string) tea.Cmd {
	return func() tea.Msg {
		fmt.Print(jsonview.OSC52Sequence(content))
		return nil
	}
}

func (m *Model) triggerAction() {
	if m.onAction == nil {
		return
	}
	if m.paused {
		m.onAction(ActionPause)
	} else {
		m.onAction(ActionResume)
	}
}

func (m *Model) clearBuffer() {
	m.events = m.events[:0]
	m.cursor = -1
	m.sources = m.sources[:0]
	for k := range m.sourceIdx {
		delete(m.sourceIdx, k)
	}
}

func (m *Model) moveCursor(delta int) {
	if len(m.events) == 0 {
		m.cursor = -1
		return
	}
	c := m.cursor + delta
	if c < 0 {
		c = 0
	}
	if c >= len(m.events) {
		c = len(m.events) - 1
	}
	m.cursor = c
	m.follow = c == len(m.events)-1
}

func nextPane(p Pane, detailVisible bool) Pane {
	order := []Pane{PaneSources, PaneStream}
	if detailVisible {
		order = append(order, PaneDetail)
	}
	for i, pp := range order {
		if pp == p {
			return order[(i+1)%len(order)]
		}
	}
	return PaneStream
}

func (m Model) streamViewportH() int {
	l := ComputeLayout(m.width, m.height, m.detailVisible)
	h := l.StreamH - 2 - 1 // borders + title row
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) helpViewportH() int {
	h := m.height - 8
	if h < 4 {
		return 4
	}
	return h
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quit {
		return ""
	}
	// Before the first WindowSizeMsg, show a minimal placeholder so
	// the TUI is not blank. The placeholder must include "LogLens" and the
	// quit hint for backward-compatible smoke tests.
	if m.width == 0 || m.height == 0 {
		return style.Title.Render("LogLens") + "\n" +
			style.Dim.Render("initializing… — press q to quit")
	}

	layout := ComputeLayout(m.width, m.height, m.detailVisible)

	src := m.viewSources(layout)
	stream := m.viewStream(layout)

	var body string
	if m.detailVisible {
		detail := m.viewDetail(layout)
		if layout.DetailBottom {
			top := lipgloss.JoinHorizontal(lipgloss.Top, src, stream)
			body = lipgloss.JoinVertical(lipgloss.Left, top, detail)
		} else {
			body = lipgloss.JoinHorizontal(lipgloss.Top, src, stream, detail)
		}
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, src, stream)
	}

	footer := m.viewFooter()
	composed := lipgloss.JoinVertical(lipgloss.Left, body, footer)

	if m.help.IsOpen() {
		return m.help.View(m.width, m.height)
	}
	return composed
}

func (m Model) viewFooter() string {
	if m.inputMode {
		return m.viewSlashPrompt()
	}
	return renderFooter(m.width, m.keys, m.paused)
}

func (m Model) viewSlashPrompt() string {
	visible := len(m.VisibleRows())
	total := len(m.events)
	cursor := "│"
	prompt := style.KeyHint.Render("/"+string(m.inputBuf)) + cursor
	hint := style.Dim.Render(fmt.Sprintf("enter to commit · esc to cancel · %d/%d rows", visible, total))
	if m.previewErr != nil {
		hint = style.Dim.Render(fmt.Sprintf("err: %v · esc to cancel", m.previewErr))
	}
	return style.Footer.Width(m.width).Render(prompt + "  " + hint)
}

func (m Model) viewSources(layout Layout) string {
	title := style.PaneTitle.Render("LogLens — Sources")
	var lines []string
	if len(m.sources) == 0 {
		lines = append(lines, style.Dim.Render("(waiting for events)"))
	} else {
		for _, s := range m.sources {
			label := truncate(s.name, layout.SourceW-5)
			lines = append(lines, style.Source.Render("• ")+style.KeyDesc.Render(label))
		}
	}
	content := strings.Join(append([]string{title}, lines...), "\n")
	totalH := layout.StreamH - 2
	if layout.DetailBottom {
		totalH += layout.DetailH
	}
	if totalH < 1 {
		totalH = 1
	}
	return paneStyle(m.focused == PaneSources).
		Width(layout.SourceW - 2).
		Height(totalH).
		Render(content)
}

func (m Model) viewStream(layout Layout) string {
	visible := m.VisibleRows()
	title := style.PaneTitle.Render(streamTitle(m.paused, m.follow, len(visible), len(m.events)))
	innerW := layout.StreamW - 2
	innerH := layout.StreamH - 2 - 1 // pane borders + title row
	if innerH < 1 {
		innerH = 1
	}
	reqID := m.selectedRequestID()
	rows := m.streamRows(visible, innerW, innerH, reqID)
	content := lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(rows, "\n"))
	return paneStyle(m.focused == PaneStream).
		Width(innerW).
		Height(layout.StreamH - 2).
		Render(content)
}

func (m Model) viewDetail(layout Layout) string {
	title := style.PaneTitle.Render("Detail")
	var bodyLines []string
	if m.cursor < 0 || m.cursor >= len(m.events) {
		bodyLines = []string{style.Dim.Render("(select a row with j/k, enter toggles pane)")}
	} else {
		ev := m.events[m.cursor]
		bodyLines = []string{
			fmt.Sprintf("%s %s", style.KeyHint.Render("time"), style.KeyDesc.Render(ev.Timestamp.Format(time.RFC3339Nano))),
			fmt.Sprintf("%s %s", style.KeyHint.Render("src "), style.KeyDesc.Render(ev.Source)),
			fmt.Sprintf("%s %s", style.KeyHint.Render("lvl "),
				lipgloss.NewStyle().Foreground(style.LevelColor(string(ev.Level))).Render(levelLabel(ev.Level))),
			"",
			ev.Raw,
		}
	}
	content := lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(bodyLines, "\n"))
	w, h := layout.DetailW-2, layout.DetailH-2
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return paneStyle(m.focused == PaneDetail).
		Width(w).Height(h).
		Render(content)
}

func paneStyle(focused bool) lipgloss.Style {
	if focused {
		return style.PaneFocused
	}
	return style.PaneBlur
}

func streamTitle(paused, following bool, visible, total int) string {
	state := ""
	switch {
	case paused:
		state = " (paused)"
	case !following:
		state = " (scrolling)"
	}
	if visible != total {
		return fmt.Sprintf("Stream — %d/%d events%s", visible, total, state)
	}
	return fmt.Sprintf("Stream — %d events%s", total, state)
}

func (m Model) streamRows(visible []event.Event, width, height int, reqID string) []string {
	if len(visible) == 0 {
		if len(m.events) == 0 {
			return []string{style.Dim.Render("waiting for events…")}
		}
		return []string{style.Dim.Render("no rows match current filter")}
	}
	cursor := m.visibleCursor(visible)
	start := cursor - height + 1
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(visible) {
		end = len(visible)
		start = end - height
		if start < 0 {
			start = 0
		}
	}
	out := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		row := formatRow(visible[i], width)
		if i == cursor {
			row = style.Selected.Width(width).Render(row)
		}
		out = append(out, row)
	}
	return out
}

// visibleCursor maps m.cursor (index into m.events) onto the filtered slice.
// Falls back to the tail when the selected event has been filtered out.
func (m Model) visibleCursor(visible []event.Event) int {
	if len(visible) == 0 || m.cursor < 0 || m.cursor >= len(m.events) {
		return len(visible) - 1
	}
	target := m.events[m.cursor]
	for i := len(visible) - 1; i >= 0; i-- {
		v := visible[i]
		if v.Timestamp.Equal(target.Timestamp) && v.Source == target.Source && v.Raw == target.Raw {
			return i
		}
	}
	return len(visible) - 1
}

func formatRow(ev event.Event, width int) string {
	ts := style.Timestamp.Render(ev.Timestamp.Format("15:04:05.000"))
	src := style.Source.Render(truncate(ev.Source, 16))
	lvl := lipgloss.NewStyle().Foreground(style.LevelColor(string(ev.Level))).Render(levelLabel(ev.Level))
	msg := ev.Raw
	row := fmt.Sprintf("%s  %s  %s  %s", ts, src, lvl, msg)
	if w := lipgloss.Width(row); width > 0 && w > width {
		overflow := w - width
		if overflow < len(msg) {
			cut := len(msg) - overflow - 1
			if cut < 0 {
				cut = 0
			}
			msg = msg[:cut] + "…"
			row = fmt.Sprintf("%s  %s  %s  %s", ts, src, lvl, msg)
		}
	}
	return row
}

func levelLabel(l event.Level) string {
	if l == "" {
		return "-----"
	}
	s := strings.ToUpper(string(l))
	if len(s) > 5 {
		s = s[:5]
	}
	for len(s) < 5 {
		s += " "
	}
	return s
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return s[:w-1] + "…"
}
