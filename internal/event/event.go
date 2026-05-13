// Package event defines the canonical LogLens event type that flows from every
// source through the merger and into the UI. Sources translate their native
// records into Event so the rest of the pipeline can stay source-agnostic.
package event

import (
	"encoding/json"
	"strings"
	"time"
)

// Level is the normalized log level. Sources should map their native severity
// onto these values; unknown severities become LevelUnknown.
type Level string

const (
	LevelUnknown Level = ""
	LevelTrace   Level = "trace"
	LevelDebug   Level = "debug"
	LevelInfo    Level = "info"
	LevelWarn    Level = "warn"
	LevelError   Level = "error"
	LevelFatal   Level = "fatal"
)

// ParseLevel normalizes common spellings (case-insensitive) into a Level.
// Unknown values return LevelUnknown.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info", "information", "notice":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error", "err":
		return LevelError
	case "fatal", "crit", "critical", "emergency", "alert":
		return LevelFatal
	default:
		return LevelUnknown
	}
}

// Event is the single record type carried by every Source.Stream channel.
// Raw is preserved verbatim so the UI can render the original line on demand.
// Fields holds structured key/value pairs extracted by the source (e.g. parsed
// JSON keys), and is nil for unstructured lines.
type Event struct {
	Timestamp time.Time
	Source    string
	Level     Level
	Fields    map[string]any
	Raw       string
}

// ParseJSONLine attempts to interpret raw as a single JSON object log line and
// returns a populated Event. If the line is not a JSON object, it falls back to
// an Event with Raw set and Fields nil. The returned Event always has Raw set
// to the trimmed input and Source set to the provided source name.
//
// Recognized field aliases:
//   - timestamp: "timestamp", "time", "ts", "@timestamp"
//   - level:     "level", "severity", "lvl"
//
// Timestamps are parsed best-effort as RFC3339; on failure, now() is used.
// The line is not required to be a JSON object; bare text comes back as Raw.
func ParseJSONLine(source, raw string, now func() time.Time) Event {
	if now == nil {
		now = time.Now
	}
	trimmed := strings.TrimRight(raw, "\r\n")
	ev := Event{
		Source:    source,
		Timestamp: now(),
		Raw:       trimmed,
	}

	s := strings.TrimSpace(trimmed)
	if len(s) == 0 || s[0] != '{' {
		return ev
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return ev
	}
	ev.Fields = obj

	for _, k := range []string{"timestamp", "time", "ts", "@timestamp"} {
		if v, ok := obj[k]; ok {
			if str, ok := v.(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
					ev.Timestamp = t
					break
				}
			}
		}
	}
	for _, k := range []string{"level", "severity", "lvl"} {
		if v, ok := obj[k]; ok {
			if str, ok := v.(string); ok {
				ev.Level = ParseLevel(str)
				break
			}
		}
	}
	return ev
}
