package file_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/source"
	filesrc "github.com/loglens/loglens/internal/source/file"
)

// drain reads up to want events with a timeout. It returns whatever it has
// when the deadline is hit so the caller can show a useful diff.
func drain(t *testing.T, ch <-chan event.Event, want int, deadline time.Duration) []event.Event {
	t.Helper()
	got := make([]event.Event, 0, want)
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	for len(got) < want {
		select {
		case ev, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, ev)
		case <-timer.C:
			return got
		}
	}
	return got
}

func TestFileSource_TailsAppendedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filesrc.New(path, filesrc.WithPollInterval(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := src.Stream(ctx)

	// First line is already in the file (fromStart default).
	first := drain(t, ch, 1, 2*time.Second)
	if len(first) != 1 || first[0].Raw != "first" {
		t.Fatalf("first line: %+v", first)
	}

	// Append more lines and verify ordered delivery.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := f.WriteString("line-" + strconv.Itoa(i) + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.Close()

	rest := drain(t, ch, 5, 3*time.Second)
	if len(rest) != 5 {
		t.Fatalf("expected 5 more lines, got %d: %+v", len(rest), rest)
	}
	for i, ev := range rest {
		want := "line-" + strconv.Itoa(i+1)
		if ev.Raw != want {
			t.Errorf("line %d: got %q want %q", i, ev.Raw, want)
		}
	}
}

func TestFileSource_HandlesRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotated.log")
	if err := os.WriteFile(path, []byte("pre-rotate-1\npre-rotate-2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filesrc.New(path, filesrc.WithPollInterval(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := src.Stream(ctx)

	pre := drain(t, ch, 2, 2*time.Second)
	if len(pre) != 2 || pre[0].Raw != "pre-rotate-1" || pre[1].Raw != "pre-rotate-2" {
		t.Fatalf("pre-rotate lines: %+v", pre)
	}

	// Rotate: rename current → .1, create a fresh file at the same path.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	// Give the tailer a tick to notice the missing path before we recreate it.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(path, []byte("post-rotate-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Append more after the rotation.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("post-rotate-2\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	post := drain(t, ch, 2, 3*time.Second)
	if len(post) != 2 {
		t.Fatalf("expected 2 post-rotate lines, got %d: %+v", len(post), post)
	}
	if post[0].Raw != "post-rotate-1" || post[1].Raw != "post-rotate-2" {
		t.Fatalf("post-rotate lines: %+v", post)
	}
}

func TestFileSource_PartialLineDeferredUntilNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.log")
	if err := os.WriteFile(path, []byte("complete\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filesrc.New(path, filesrc.WithPollInterval(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := src.Stream(ctx)

	if first := drain(t, ch, 1, 2*time.Second); len(first) != 1 || first[0].Raw != "complete" {
		t.Fatalf("first line: %+v", first)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("half "); err != nil {
		t.Fatal(err)
	}
	// Give the tailer time to read the partial fragment but expect no event.
	time.Sleep(150 * time.Millisecond)
	select {
	case ev := <-ch:
		t.Fatalf("did not expect event for partial line, got: %+v", ev)
	default:
	}
	if _, err := f.WriteString("done\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	rest := drain(t, ch, 1, 2*time.Second)
	if len(rest) != 1 || rest[0].Raw != "half done" {
		t.Fatalf("expected combined line, got: %+v", rest)
	}
}

func TestFileSource_URIRegistration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reg.log")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := source.Open("file://" + path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Name() != "file://"+path {
		t.Fatalf("name: %q", s.Name())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := drain(t, s.Stream(ctx), 1, 2*time.Second)
	if len(got) != 1 || got[0].Raw != "hello" {
		t.Fatalf("got: %+v", got)
	}
}
