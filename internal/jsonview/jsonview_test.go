package jsonview

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func parseObj(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return m
}

func TestNew_NestedObject_ExpandedRoot(t *testing.T) {
	raw := `{"user":{"id":42,"name":"matt"},"msg":"hi"}`
	m := New(parseObj(t, raw), raw)
	got := m.Lines()
	want := []string{
		"▾ ",
		"    msg: \"hi\"",
		"  ▸ user {2}",
	}
	if !equalLines(got, want) {
		t.Fatalf("nested object lines:\n got=%q\nwant=%q", got, want)
	}
}

func TestNew_ExpandChildObject(t *testing.T) {
	raw := `{"user":{"id":42,"name":"matt"}}`
	m := New(parseObj(t, raw), raw)
	// cursor=0 is root; move to child (user) and expand.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	got := m.Lines()
	want := []string{
		"▾ ",
		"  ▾ user",
		"      id: 42",
		"      name: \"matt\"",
	}
	if !equalLines(got, want) {
		t.Fatalf("expanded child:\n got=%q\nwant=%q", got, want)
	}
}

func TestNew_Array(t *testing.T) {
	raw := `{"tags":["alpha","beta","gamma"]}`
	m := New(parseObj(t, raw), raw)
	// collapsed array: "▸ tags [3]"
	got := m.Lines()
	if got[1] != "  ▸ tags [3]" {
		t.Fatalf("collapsed array label: %q", got[1])
	}
	// expand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	got = m.Lines()
	want := []string{
		"▾ ",
		"  ▾ tags",
		"      [0]: \"alpha\"",
		"      [1]: \"beta\"",
		"      [2]: \"gamma\"",
	}
	if !equalLines(got, want) {
		t.Fatalf("expanded array:\n got=%q\nwant=%q", got, want)
	}
}

func TestNew_MixedScalars(t *testing.T) {
	raw := `{"s":"hi","n":42,"f":3.14,"b":true,"z":null}`
	m := New(parseObj(t, raw), raw)
	got := m.Lines()
	// Map keys are sorted: b, f, n, s, z
	want := []string{
		"▾ ",
		"    b: true",
		"    f: 3.14",
		"    n: 42",
		"    s: \"hi\"",
		"    z: null",
	}
	if !equalLines(got, want) {
		t.Fatalf("mixed scalars:\n got=%q\nwant=%q", got, want)
	}
}

func TestUpdate_CollapseRoot(t *testing.T) {
	raw := `{"k":1}`
	m := New(parseObj(t, raw), raw)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got := m.Lines()
	if len(got) != 1 || !strings.HasPrefix(got[0], "▸ ") {
		t.Fatalf("expected collapsed root, got %q", got)
	}
}

func TestUpdate_YEmitsClipboardWrite(t *testing.T) {
	raw := `{"k":1}`
	m := New(parseObj(t, raw), raw)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from y press")
	}
	msg := cmd()
	cw, ok := msg.(ClipboardWriteMsg)
	if !ok {
		t.Fatalf("expected ClipboardWriteMsg, got %T", msg)
	}
	if cw.Content != raw {
		t.Fatalf("clipboard content: got %q want %q", cw.Content, raw)
	}
}

func TestOSC52Sequence(t *testing.T) {
	got := OSC52Sequence("hello")
	wantEnc := base64.StdEncoding.EncodeToString([]byte("hello"))
	want := "\x1b]52;c;" + wantEnc + "\x07"
	if got != want {
		t.Fatalf("OSC52: got %q want %q", got, want)
	}
}

func TestNewRaw_FallbackForNonJSON(t *testing.T) {
	m := NewRaw("plain text line")
	if !m.IsRaw() {
		t.Fatal("expected raw mode")
	}
	got := m.Lines()
	if len(got) != 1 || got[0] != "plain text line" {
		t.Fatalf("raw lines: %q", got)
	}
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
