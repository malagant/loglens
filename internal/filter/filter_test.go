package filter

import (
	"strings"
	"testing"

	"github.com/loglens/loglens/internal/event"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"foo", []string{"foo"}},
		{"  foo   bar ", []string{"foo", "bar"}},
		{"level=error source=k8s://ns/pod", []string{"level=error", "source=k8s://ns/pod"}},
		{"/partial substring/", []string{"/partial substring/"}},
		{"foo /spaces here/ bar", []string{"foo", "/spaces here/", "bar"}},
		{"!info", []string{"!info"}},
		{"!level=error", []string{"!level=error"}},
		{"!/needle one/ baz", []string{"!/needle one/", "baz"}},
		{"!", []string{"!"}},
		{"/unterminated needle", []string{"/unterminated needle"}},
	}
	for _, tc := range cases {
		got := tokenize(tc.in)
		if !equalSlices(got, tc.want) {
			t.Errorf("tokenize(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestEmptyQueryMatchesEverything(t *testing.T) {
	q, err := Parse("")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !q.Empty() {
		t.Fatal("expected empty query")
	}
	if !q.Match(event.Event{}) {
		t.Fatal("empty query must match any event")
	}
}

func TestMatcher(t *testing.T) {
	mk := func(src, raw string, lvl event.Level, fields map[string]any) event.Event {
		return event.Event{Source: src, Raw: raw, Level: lvl, Fields: fields}
	}

	errEv := mk("file:///var/log/app.log", `{"msg":"boom","user_id":42}`, event.LevelError,
		map[string]any{"msg": "boom", "user_id": float64(42)})
	infoEv := mk("k8s://prod/ingress-7d", "request id=abc handled", event.LevelInfo, nil)
	plainEv := mk("file:///tmp/raw.log", "no structure here", event.LevelUnknown, nil)

	cases := []struct {
		name     string
		query    string
		ev       event.Event
		expected bool
	}{
		{"level matches", "level=error", errEv, true},
		{"level rejects", "level=error", infoEv, false},
		{"level case insensitive", "level=ERROR", errEv, true},
		{"level alias warn", "level=warning", mk("x", "x", event.LevelWarn, nil), true},

		{"source substring", "source=k8s://prod", infoEv, true},
		{"source rejects", "source=k8s://prod", errEv, false},

		{"field exact value", "field.user_id=42", errEv, true},
		{"field missing key", "field.user_id=42", infoEv, false},
		{"field string contains", "field.msg=boo", errEv, true},
		{"field nil fields rejects", "field.msg=x", plainEv, false},

		{"substring slash", "/handled/", infoEv, true},
		{"substring slash with space", "/id=abc handled/", infoEv, true},
		{"substring slash misses", "/handled/", errEv, false},
		{"bareword substring", "boom", errEv, true},
		{"bareword misses", "boom", infoEv, false},

		{"negation level", "!level=error", infoEv, true},
		{"negation level rejects matching", "!level=error", errEv, false},
		{"negation substring", "!handled", errEv, true},
		{"negation slash", "!/handled/", infoEv, false},

		{"conjunction matches", "level=info source=k8s", infoEv, true},
		{"conjunction one fails", "level=info source=file", infoEv, false},
		{"conjunction with negation", "source=k8s !error", infoEv, true},

		{"empty matches", "", errEv, true},
		{"whitespace only matches", "   ", infoEv, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := Parse(tc.query)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.query, err)
			}
			got := q.Match(tc.ev)
			if got != tc.expected {
				t.Errorf("query %q on %+v = %v, want %v (parsed: %s)", tc.query, tc.ev, got, tc.expected, q.String())
			}
		})
	}
}

func TestQueryString(t *testing.T) {
	q, err := Parse("level=error source=k8s /needle here/ !info field.user_id=42")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	s := q.String()
	for _, want := range []string{"level=error", "source=k8s", "/needle here/", "!info", "field.user_id=42"} {
		if !strings.Contains(s, want) {
			t.Errorf("String()=%q missing %q", s, want)
		}
	}
}

func equalSlices(a, b []string) bool {
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
