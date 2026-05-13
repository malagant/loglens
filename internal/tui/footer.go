package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/loglens/loglens/internal/tui/style"
)

// renderFooter builds the single-row keymap bar. It greedily packs the
// highest-priority (Basic) bindings and drops trailing ones when they don't
// fit, so the bar is always exactly `width` columns — never wraps.
func renderFooter(width int, km Keymap, paused bool) string {
	if width < 1 {
		return ""
	}
	segs := buildFooterSegs(km, paused)
	const sep = "  "
	visible := packSegs(segs, width, len(sep))
	return style.Footer.Width(width).Render(strings.Join(visible, sep))
}

type footerSeg struct {
	rendered string
	w        int // visible (ANSI-stripped) width
}

func buildFooterSegs(km Keymap, paused bool) []footerSeg {
	var out []footerSeg
	for _, b := range km.Basic() {
		key := b.Display()
		desc := b.Help
		if b.Matches("p") && paused {
			desc = "resume"
		}
		r := style.KeyHint.Render(key) + style.KeySep.Render(":") + style.KeyDesc.Render(desc)
		out = append(out, footerSeg{rendered: r, w: lipgloss.Width(r)})
	}
	return out
}

// packSegs greedily packs segments into the inner width (outer width minus
// the 2-col horizontal padding that style.Footer adds).
func packSegs(segs []footerSeg, outerW, sepW int) []string {
	inner := outerW - 2 // style.Footer padding: 1 each side
	if inner < 1 {
		inner = outerW
	}
	var out []string
	used := 0
	for i, s := range segs {
		extra := s.w
		if i > 0 {
			extra += sepW
		}
		if used+extra > inner {
			break
		}
		out = append(out, s.rendered)
		used += extra
	}
	return out
}
