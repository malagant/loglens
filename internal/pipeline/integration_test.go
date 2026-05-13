package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/pipeline"
	filesrc "github.com/loglens/loglens/internal/source/file"
)

// TestIntegration_FileSourceThroughMerger writes lines into a temp file and
// asserts they arrive on the merged channel in order, including across a
// rotation. This is the end-to-end shape required by SPA-13.
func TestIntegration_FileSourceThroughMerger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "integration.log")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filesrc.New(path, filesrc.WithPollInterval(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := pipeline.New(1024)
	defer m.Close()
	m.Add(ctx, src.Stream(ctx))

	// Write a first batch.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := f.WriteString("pre-" + strconv.Itoa(i) + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.Close()

	got := collect(t, m.Events(), 5, 3*time.Second)
	for i, ev := range got {
		want := "pre-" + strconv.Itoa(i)
		if ev.Raw != want {
			t.Fatalf("pre line %d: got %q want %q", i, ev.Raw, want)
		}
	}

	// Rotate.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	f, err = os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := f.WriteString("post-" + strconv.Itoa(i) + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.Close()

	got = collect(t, m.Events(), 5, 3*time.Second)
	for i, ev := range got {
		want := "post-" + strconv.Itoa(i)
		if ev.Raw != want {
			t.Fatalf("post line %d: got %q want %q", i, ev.Raw, want)
		}
	}
}

func collect(t *testing.T, ch <-chan event.Event, n int, timeout time.Duration) []event.Event {
	t.Helper()
	out := make([]event.Event, 0, n)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for len(out) < n {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-timer.C:
			t.Fatalf("timed out after %d events: %+v", len(out), out)
		}
	}
	return out
}
