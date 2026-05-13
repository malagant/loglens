// Package correlate finds a request id inside a structured log event and marks
// every event in a list that shares that id. The default key set covers the
// common spellings (request_id, trace_id, x-request-id, requestId) and is
// overridable via Parse so users can plumb their own via the --correlate flag.
package correlate

import (
	"strconv"
	"strings"

	"github.com/loglens/loglens/internal/event"
)

// DefaultKeys is the lookup order applied when the user has not customized
// --correlate. The first non-empty value wins, so request_id beats trace_id
// when both are present on the same event.
var DefaultKeys = []string{"request_id", "trace_id", "x-request-id", "requestId"}

// Detector looks up the configured keys on an Event.Fields map.
// The zero value is unusable; construct with Default or Parse.
type Detector struct {
	keys []string
}

// Default returns a Detector preloaded with DefaultKeys.
func Default() Detector { return Detector{keys: append([]string(nil), DefaultKeys...)} }

// Parse builds a Detector from a comma-separated key list. Whitespace around
// each key is trimmed; empty entries are dropped. An empty or all-empty input
// falls back to DefaultKeys so a user typing --correlate= cannot disable
// detection unintentionally.
func Parse(csv string) Detector {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return Default()
	}
	return Detector{keys: out}
}

// Keys returns the active key list. Callers must not mutate the slice.
func (d Detector) Keys() []string { return d.keys }

// ID returns the first non-empty value among the configured keys on the
// event's Fields map, or "" when the event has no Fields (non-JSON line) or
// none of the keys are present.
//
// Recognized value shapes:
//   - string   → trimmed
//   - number   → formatted (json.Number preserved; float64 rendered without
//     scientific notation for typical integer ids)
//   - bool/null/object/array → ignored (request ids should be scalars)
func (d Detector) ID(ev event.Event) string {
	if ev.Fields == nil {
		return ""
	}
	for _, k := range d.keys {
		raw, ok := ev.Fields[k]
		if !ok {
			continue
		}
		if s := stringify(raw); s != "" {
			return s
		}
	}
	return ""
}

// Mark returns one bool per event. Entry i is true when events[i] carries a
// request id equal to target. An empty target marks nothing.
func (d Detector) Mark(events []event.Event, target string) []bool {
	out := make([]bool, len(events))
	if target == "" {
		return out
	}
	for i, ev := range events {
		if d.ID(ev) == target {
			out[i] = true
		}
	}
	return out
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}
