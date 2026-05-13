package event

import (
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{
		"INFO":        LevelInfo,
		"warn":        LevelWarn,
		"warning":     LevelWarn,
		"ERROR":       LevelError,
		"err":         LevelError,
		"critical":    LevelFatal,
		"  debug  ":   LevelDebug,
		"trace":       LevelTrace,
		"information": LevelInfo,
		"nope":        LevelUnknown,
		"":            LevelUnknown,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseJSONLine_StructuredJSON(t *testing.T) {
	now := func() time.Time { return time.Unix(0, 0).UTC() }
	line := `{"timestamp":"2026-05-13T12:34:56Z","level":"warn","msg":"disk almost full","host":"web-1"}`
	ev := ParseJSONLine("file://var/log/app", line, now)
	if ev.Source != "file://var/log/app" {
		t.Fatalf("source: %q", ev.Source)
	}
	if ev.Level != LevelWarn {
		t.Fatalf("level: %q", ev.Level)
	}
	if !ev.Timestamp.Equal(time.Date(2026, 5, 13, 12, 34, 56, 0, time.UTC)) {
		t.Fatalf("timestamp: %v", ev.Timestamp)
	}
	if ev.Fields["msg"] != "disk almost full" {
		t.Fatalf("fields[msg]: %v", ev.Fields["msg"])
	}
	if ev.Raw != line {
		t.Fatalf("raw: %q", ev.Raw)
	}
}

func TestParseJSONLine_NonJSON(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ev := ParseJSONLine("file://x", "  plain text line\n", func() time.Time { return fixed })
	if ev.Fields != nil {
		t.Fatalf("fields should be nil, got: %v", ev.Fields)
	}
	if ev.Level != LevelUnknown {
		t.Fatalf("level: %q", ev.Level)
	}
	if !ev.Timestamp.Equal(fixed) {
		t.Fatalf("timestamp: %v", ev.Timestamp)
	}
	if ev.Raw != "  plain text line" {
		t.Fatalf("raw should be trimmed of CRLF, got: %q", ev.Raw)
	}
}

func TestParseJSONLine_InvalidJSONFallsBack(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ev := ParseJSONLine("file://x", `{"unterminated":`, func() time.Time { return fixed })
	if ev.Fields != nil {
		t.Fatalf("expected nil fields on malformed JSON, got: %v", ev.Fields)
	}
	if !ev.Timestamp.Equal(fixed) {
		t.Fatalf("expected fallback timestamp, got: %v", ev.Timestamp)
	}
}

func TestParseJSONLine_TSAliases(t *testing.T) {
	want := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	for _, key := range []string{"timestamp", "time", "ts", "@timestamp"} {
		line := `{"` + key + `":"2026-05-13T10:00:00Z","severity":"error"}`
		ev := ParseJSONLine("s", line, nil)
		if !ev.Timestamp.Equal(want) {
			t.Errorf("%s: got %v want %v", key, ev.Timestamp, want)
		}
		if ev.Level != LevelError {
			t.Errorf("%s: level %q", key, ev.Level)
		}
	}
}
