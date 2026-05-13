// Package style centralizes the lipgloss palette and styles used across the
// LogLens TUI. Keeping every color and border here lets us re-skin the UI or
// add a high-contrast mode without hunting through view code.
package style

import "github.com/charmbracelet/lipgloss"

// Palette.
var (
	Primary = lipgloss.Color("#7D56F4")
	Text    = lipgloss.Color("#FAFAFA")
	Muted   = lipgloss.Color("#9CA3AF")
	Faint   = lipgloss.Color("#6B7280")
	Accent  = lipgloss.Color("#FFD866")

	LevelDebug = lipgloss.Color("#A9DC76")
	LevelInfo  = lipgloss.Color("#7DD3FC")
	LevelWarn  = lipgloss.Color("#FFD866")
	LevelError = lipgloss.Color("#FF6188")
	LevelFatal = lipgloss.Color("#FF6188")
)

// Styles. These are values — copy semantics keep them safe to derive from
// (e.g. .Width(n)) without mutating the package-level originals.
var (
	Title = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	Dim   = lipgloss.NewStyle().Foreground(Muted)

	PaneTitle = lipgloss.NewStyle().Bold(true).Foreground(Primary).Padding(0, 1)

	PaneFocused = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary)
	PaneBlur = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Faint)

	Footer = lipgloss.NewStyle().
		Foreground(Text).
		Background(lipgloss.Color("#1F1F23")).
		Padding(0, 1)

	KeyHint = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	KeyDesc = lipgloss.NewStyle().Foreground(Text)
	KeySep  = lipgloss.NewStyle().Foreground(Faint)

	HelpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 2).
		Background(lipgloss.Color("#1F1F23"))
	HelpHeader = lipgloss.NewStyle().Bold(true).Foreground(Primary).Underline(true)

	Timestamp = lipgloss.NewStyle().Foreground(Faint)
	Source    = lipgloss.NewStyle().Foreground(Primary)
	Selected  = lipgloss.NewStyle().Background(lipgloss.Color("#2D2A2E")).Foreground(Text)
)

// LevelColor maps a normalized log level string (see internal/event.Level) to
// a palette color. Unknown levels return Muted so the row still renders.
func LevelColor(level string) lipgloss.Color {
	switch level {
	case "debug", "trace":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return Muted
	}
}
