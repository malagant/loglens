package tui

import "testing"

// Snapshot-style assertions across width branches. We assert geometry (the
// contract view code relies on), not rendered strings (those churn on styling).
func TestComputeLayoutWidthBranches(t *testing.T) {
	cases := []struct {
		name          string
		w, h          int
		detail        bool
		wantSrcW      int
		wantStreamW   int
		wantDetailW   int
		wantBottom    bool
		wantDetailH   int
	}{
		{
			name: "wide 160 detail right",
			w: 160, h: 40, detail: true,
			wantSrcW: 22, wantStreamW: 160 - 22 - 32, wantDetailW: 32,
			wantBottom: false, wantDetailH: 39,
		},
		{
			name: "exactly 100 detail right",
			w: 100, h: 40, detail: true,
			wantSrcW: 22, wantStreamW: 100 - 22 - 32, wantDetailW: 32,
			wantBottom: false, wantDetailH: 39,
		},
		{
			name: "narrow 90 detail stacked",
			w: 90, h: 30, detail: true,
			wantSrcW: 22, wantStreamW: 68, wantDetailW: 68,
			wantBottom: true, wantDetailH: 29 / 3,
		},
		{
			name: "80-col detail stacked",
			w: 80, h: 24, detail: true,
			wantSrcW: 22, wantStreamW: 58, wantDetailW: 58,
			wantBottom: true, wantDetailH: 23 / 3,
		},
		{
			name: "80-col no detail",
			w: 80, h: 24, detail: false,
			wantSrcW: 22, wantStreamW: 58, wantDetailW: 0,
			wantBottom: false, wantDetailH: 0,
		},
		{
			name: "tiny 50 collapses source",
			w: 50, h: 20, detail: false,
			wantSrcW: 12, wantStreamW: 38, wantDetailW: 0,
			wantBottom: false, wantDetailH: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeLayout(tc.w, tc.h, tc.detail)

			if got.SourceW != tc.wantSrcW {
				t.Errorf("SourceW = %d, want %d", got.SourceW, tc.wantSrcW)
			}
			if got.StreamW != tc.wantStreamW {
				t.Errorf("StreamW = %d, want %d", got.StreamW, tc.wantStreamW)
			}
			if got.DetailW != tc.wantDetailW {
				t.Errorf("DetailW = %d, want %d", got.DetailW, tc.wantDetailW)
			}
			if got.DetailBottom != tc.wantBottom {
				t.Errorf("DetailBottom = %v, want %v", got.DetailBottom, tc.wantBottom)
			}
			if tc.detail && got.DetailH != tc.wantDetailH {
				t.Errorf("DetailH = %d, want %d", got.DetailH, tc.wantDetailH)
			}
			if got.FooterH != 1 {
				t.Errorf("FooterH = %d, want 1", got.FooterH)
			}

			// Vertical budget: stream + footer + (bottom detail if present) == h.
			detailRows := 0
			if got.DetailBottom {
				detailRows = got.DetailH
			}
			if got.StreamH+got.FooterH+detailRows != tc.h {
				t.Errorf("vertical sum %d+%d+%d = %d, want %d",
					got.StreamH, got.FooterH, detailRows, got.StreamH+got.FooterH+detailRows, tc.h)
			}

			// Horizontal budget: source + stream + (right detail if any) == w.
			horiz := got.SourceW + got.StreamW
			if !got.DetailBottom && tc.detail {
				horiz += got.DetailW
			}
			if horiz != tc.w {
				t.Errorf("horizontal sum %d = %d, want %d", horiz, horiz, tc.w)
			}
		})
	}
}

func TestComputeLayoutDegenerate(t *testing.T) {
	got := ComputeLayout(0, 0, true)
	if got.Width < 1 || got.Height < 2 {
		t.Fatalf("expected clamped dims, got %dx%d", got.Width, got.Height)
	}
	if got.FooterH != 1 {
		t.Fatalf("FooterH = %d, want 1", got.FooterH)
	}
	if got.StreamH < 1 {
		t.Fatalf("StreamH should be >= 1, got %d", got.StreamH)
	}
}
