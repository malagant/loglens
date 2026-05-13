package tui

import "testing"

func TestDefaultKeymapCovers(t *testing.T) {
	want := []struct {
		name string
		key  string
		b    Binding
	}{
		{"up", "k", DefaultKeymap.Up},
		{"up alias", "up", DefaultKeymap.Up},
		{"down", "j", DefaultKeymap.Down},
		{"search", "/", DefaultKeymap.Search},
		{"help", "?", DefaultKeymap.Help},
		{"quit", "q", DefaultKeymap.Quit},
		{"quit ctrl+c", "ctrl+c", DefaultKeymap.Quit},
		{"pause", "p", DefaultKeymap.Pause},
		{"clear", "c", DefaultKeymap.Clear},
		{"detail", "enter", DefaultKeymap.Detail},
		{"cycle", "tab", DefaultKeymap.CyclePane},
		{"top", "g", DefaultKeymap.JumpTop},
		{"bottom", "G", DefaultKeymap.JumpBot},
		{"page down", "ctrl+d", DefaultKeymap.PageDown},
		{"page up", "ctrl+u", DefaultKeymap.PageUp},
		{"follow", "f", DefaultKeymap.Follow},
	}
	for _, w := range want {
		if !w.b.Matches(w.key) {
			t.Errorf("%s: %q should match binding", w.name, w.key)
		}
	}
}

func TestKeymapCategories(t *testing.T) {
	basic := DefaultKeymap.Basic()
	if len(basic) < 8 {
		t.Fatalf("expected at least 8 basic bindings, got %d", len(basic))
	}
	for _, b := range basic {
		if b.Category != CategoryBasic {
			t.Errorf("Basic() returned non-basic binding: %v", b)
		}
	}
	power := DefaultKeymap.Power()
	if len(power) < 4 {
		t.Fatalf("expected at least 4 power bindings, got %d", len(power))
	}
	for _, b := range power {
		if b.Category != CategoryPower {
			t.Errorf("Power() returned non-power binding: %v", b)
		}
	}
}

func TestKeymapAllOrdered(t *testing.T) {
	all := DefaultKeymap.All()
	if len(all) < 13 {
		t.Fatalf("expected >= 13 total bindings, got %d", len(all))
	}
	// First three must be navigation + search; quit must appear before detail
	// so narrow footers always show it.
	if all[0].Help != "up" || all[1].Help != "down" || all[2].Help != "search" {
		t.Fatalf("first three should be up/down/search, got %q/%q/%q",
			all[0].Help, all[1].Help, all[2].Help)
	}
	quitIdx, detailIdx := -1, -1
	for i, b := range all {
		switch b.Help {
		case "quit":
			quitIdx = i
		case "detail":
			detailIdx = i
		}
	}
	if quitIdx < 0 || detailIdx < 0 {
		t.Fatal("quit or detail binding missing from All()")
	}
	if quitIdx > detailIdx {
		t.Fatalf("quit (%d) must precede detail (%d) for 80-col footer packing", quitIdx, detailIdx)
	}
}

func TestBindingMatchesEmpty(t *testing.T) {
	b := DefaultKeymap.Quit
	if b.Matches("") {
		t.Fatal("empty key should never match")
	}
	if b.Matches("z") {
		t.Fatal("unrelated key should not match quit")
	}
}

func TestBindingDisplay(t *testing.T) {
	if got := DefaultKeymap.Quit.Display(); got != "q" {
		t.Fatalf("expected canonical display 'q', got %q", got)
	}
	empty := Binding{}
	if got := empty.Display(); got != "" {
		t.Fatalf("expected empty display for empty binding, got %q", got)
	}
}
