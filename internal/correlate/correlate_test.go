package correlate

import (
	"reflect"
	"testing"
	"time"

	"github.com/loglens/loglens/internal/event"
)

func TestDefault_KeyOrder(t *testing.T) {
	d := Default()
	want := []string{"request_id", "trace_id", "x-request-id", "requestId"}
	if !reflect.DeepEqual(d.Keys(), want) {
		t.Fatalf("default keys: got %v want %v", d.Keys(), want)
	}
}

func TestParse_OverrideAndFallback(t *testing.T) {
	d := Parse(" foo , bar ,, ")
	if !reflect.DeepEqual(d.Keys(), []string{"foo", "bar"}) {
		t.Fatalf("parse override: %v", d.Keys())
	}
	if !reflect.DeepEqual(Parse("").Keys(), DefaultKeys) {
		t.Fatalf("empty input should fall back to defaults, got %v", Parse("").Keys())
	}
	if !reflect.DeepEqual(Parse(", , ,").Keys(), DefaultKeys) {
		t.Fatalf("all-empty input should fall back to defaults")
	}
}

func TestID_FirstNonEmptyWins(t *testing.T) {
	d := Default()
	ev := event.Event{Fields: map[string]any{
		"trace_id":   "trace-9",
		"request_id": "req-7",
		"requestId":  "rid-3",
	}}
	if got := d.ID(ev); got != "req-7" {
		t.Fatalf("priority: got %q want req-7", got)
	}
}

func TestID_NumericAndMissing(t *testing.T) {
	d := Default()
	if got := d.ID(event.Event{Fields: map[string]any{"request_id": float64(42)}}); got != "42" {
		t.Fatalf("numeric id: %q", got)
	}
	if got := d.ID(event.Event{Fields: map[string]any{"unrelated": "x"}}); got != "" {
		t.Fatalf("missing key should be empty, got %q", got)
	}
	if got := d.ID(event.Event{}); got != "" {
		t.Fatalf("nil fields should be empty, got %q", got)
	}
}

func TestMark_AcrossTwoSourcesSharingID(t *testing.T) {
	now := time.Now().UTC()
	evs := []event.Event{
		// source A
		{Source: "file://a.log", Timestamp: now, Fields: map[string]any{"request_id": "abc", "msg": "start"}},
		{Source: "file://a.log", Timestamp: now.Add(1 * time.Millisecond), Fields: map[string]any{"request_id": "xyz", "msg": "other"}},
		// source B
		{Source: "k8s://web/0", Timestamp: now.Add(2 * time.Millisecond), Fields: map[string]any{"trace_id": "abc", "msg": "handled"}},
		{Source: "k8s://web/0", Timestamp: now.Add(3 * time.Millisecond), Raw: "non-json line"}, // no fields
		// source A again with an alternate key
		{Source: "file://a.log", Timestamp: now.Add(4 * time.Millisecond), Fields: map[string]any{"x-request-id": "abc"}},
	}
	d := Default()
	got := d.Mark(evs, "abc")
	want := []bool{true, false, true, false, true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mark across sources: got %v want %v", got, want)
	}
}

func TestMark_EmptyTarget(t *testing.T) {
	d := Default()
	got := d.Mark([]event.Event{{Fields: map[string]any{"request_id": "abc"}}}, "")
	if got[0] {
		t.Fatalf("empty target should mark nothing")
	}
}

func TestParse_CustomKeyMatches(t *testing.T) {
	d := Parse("session_id")
	ev := event.Event{Fields: map[string]any{"request_id": "ignored", "session_id": "sess-1"}}
	if got := d.ID(ev); got != "sess-1" {
		t.Fatalf("custom keys should ignore defaults; got %q", got)
	}
}
