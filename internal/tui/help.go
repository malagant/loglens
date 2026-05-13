package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/loglens/loglens/internal/tui/style"
)

// helpModel holds the `?` overlay state: open/closed flag and scroll offset.
type helpModel struct {
	open   bool
	scroll int
	km     Keymap
}

func newHelp(km Keymap) helpModel { return helpModel{km: km} }

func (h *helpModel) Open()   { h.open = true; h.scroll = 0 }
func (h *helpModel) Close()  { h.open = false }
func (h *helpModel) IsOpen() bool { return h.open }

func (h *helpModel) Toggle() {
	if h.open {
		h.Close()
	} else {
		h.Open()
	}
}

func (h *helpModel) ScrollDown(viewportH int) {
	max := h.lineCount() - viewportH
	if max < 0 {
		max = 0
	}
	h.scroll++
	if h.scroll > max {
		h.scroll = max
	}
}

func (h *helpModel) ScrollUp() {
	h.scroll--
	if h.scroll < 0 {
		h.scroll = 0
	}
}

func (h *helpModel) lineCount() int { return len(h.styledBody()) }

// View renders the overlay centered over the canvas. Returns "" when closed.
func (h *helpModel) View(canvasW, canvasH int) string {
	if !h.open {
		return ""
	}
	width := canvasW * 6 / 10
	if width > 64 {
		width = 64
	}
	if width < 30 {
		width = canvasW - 4
	}
	if width < 20 {
		width = 20
	}

	body := h.styledBody()
	total := len(body)
	innerH := canvasH - 4
	if innerH < 4 {
		innerH = 4
	}
	if innerH > total {
		innerH = total
	}

	start := h.scroll
	if start > total-innerH {
		start = total - innerH
	}
	if start < 0 {
		start = 0
	}
	end := start + innerH
	if end > total {
		end = total
	}

	header := style.HelpHeader.Render("LogLens — keybindings")
	scrollHint := ""
	if total > innerH {
		scrollHint = "  " + style.Dim.Render(fmt.Sprintf("%d–%d/%d  j/k scroll", start+1, end, total))
	}
	content := lipgloss.JoinVertical(lipgloss.Left,
		header+scrollHint,
		"",
		strings.Join(body[start:end], "\n"),
	)
	box := style.HelpBox.Width(width).Render(content)
	return lipgloss.Place(canvasW, canvasH, lipgloss.Center, lipgloss.Center, box)
}

func (h *helpModel) styledBody() []string {
	var lines []string
	lines = append(lines, style.Title.Render("Basic"))
	for _, b := range h.km.Basic() {
		lines = append(lines, fmt.Sprintf("  %s  %s",
			style.KeyHint.Render(strings.Join(b.Keys, " / ")),
			style.KeyDesc.Render(b.Help),
		))
	}
	lines = append(lines, "")
	lines = append(lines, style.Title.Render("Power user"))
	for _, b := range h.km.Power() {
		lines = append(lines, fmt.Sprintf("  %s  %s",
			style.KeyHint.Render(strings.Join(b.Keys, " / ")),
			style.KeyDesc.Render(b.Help),
		))
	}
	lines = append(lines, "")
	lines = append(lines, style.Dim.Render("Press ? or esc to close."))
	return lines
}
