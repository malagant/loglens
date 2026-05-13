package tui

// Layout is the computed geometry of the three-pane TUI shell for a given
// terminal size. View code reads these dimensions to size each pane without
// repeating width math; tests assert the dimensions across width branches.
type Layout struct {
	Width  int
	Height int

	// Footer is rendered on the bottom-most row. Always 1 row tall.
	FooterH int

	// Source list (left column).
	SourceW int

	// Center column containing the merged stream.
	StreamW int

	// Detail pane. When DetailBottom is true, the detail pane is stacked
	// below the stream and spans (Width - SourceW). Otherwise it sits to the
	// right of the stream and is DetailW wide.
	DetailW      int
	DetailBottom bool

	// Heights of the stream and detail panes (full pane height incl. border).
	StreamH int
	DetailH int

	// DetailVisible mirrors the input so view code can branch on a single
	// field instead of re-deriving it.
	DetailVisible bool
}

const (
	narrowWidth       = 100
	tinyWidth         = 60
	minSourceW        = 12
	defaultSrcW       = 22
	defaultDetW       = 32
	minStreamW        = 20
	bottomDetailRatio = 3
)

// ComputeLayout derives a Layout from the terminal width, height, and whether
// the detail pane is currently visible. It clamps degenerate sizes so the
// view always renders something coherent even on tiny terminals.
func ComputeLayout(width, height int, detailVisible bool) Layout {
	if width < 1 {
		width = 1
	}
	if height < 2 {
		height = 2
	}

	contentH := height - 1
	if contentH < 1 {
		contentH = 1
	}

	sourceW := defaultSrcW
	if width < tinyWidth {
		sourceW = minSourceW
	}
	if sourceW > width/2 {
		sourceW = width / 2
	}
	if sourceW < minSourceW && width >= minSourceW*2 {
		sourceW = minSourceW
	}

	out := Layout{
		Width:         width,
		Height:        height,
		FooterH:       1,
		SourceW:       sourceW,
		StreamH:       contentH,
		DetailVisible: detailVisible,
	}

	if !detailVisible {
		out.StreamW = width - sourceW
		return out
	}

	if width < narrowWidth {
		out.DetailBottom = true
		out.StreamW = width - sourceW
		out.DetailW = width - sourceW
		detailH := contentH / bottomDetailRatio
		if detailH < 1 {
			detailH = 1
		}
		out.DetailH = detailH
		out.StreamH = contentH - detailH
		if out.StreamH < 1 {
			out.StreamH = 1
		}
		return out
	}

	detailW := defaultDetW
	if width-sourceW-minStreamW < detailW {
		detailW = width - sourceW - minStreamW
	}
	if detailW < 16 {
		detailW = 16
	}
	out.DetailW = detailW
	out.StreamW = width - sourceW - detailW
	out.DetailH = contentH
	return out
}
