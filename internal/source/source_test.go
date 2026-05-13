package source

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/loglens/loglens/internal/event"
)

func TestParseURI_BarePathBecomesFile(t *testing.T) {
	u, err := ParseURI("/var/log/system.log")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "file" {
		t.Fatalf("scheme: %q", u.Scheme)
	}
	p, err := FilePath(u)
	if err != nil {
		t.Fatal(err)
	}
	if p != "/var/log/system.log" {
		t.Fatalf("path: %q", p)
	}
}

func TestParseURI_FileSchemeAbsolute(t *testing.T) {
	u, err := ParseURI("file:///tmp/app.log")
	if err != nil {
		t.Fatal(err)
	}
	p, err := FilePath(u)
	if err != nil {
		t.Fatal(err)
	}
	if p != "/tmp/app.log" {
		t.Fatalf("path: %q", p)
	}
}

func TestParseURI_FileSchemeRelative(t *testing.T) {
	u, err := ParseURI("file://logs/today.log")
	if err != nil {
		t.Fatal(err)
	}
	p, err := FilePath(u)
	if err != nil {
		t.Fatal(err)
	}
	if p != "logs/today.log" {
		t.Fatalf("path: %q", p)
	}
}

func TestParseURI_Empty(t *testing.T) {
	if _, err := ParseURI(""); err == nil {
		t.Fatal("expected error on empty URI")
	}
}

// fakeSource is a minimal Source used to exercise the registry.
type fakeSource struct {
	name string
	ch   chan event.Event
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Stream(ctx context.Context) <-chan event.Event {
	go func() {
		<-ctx.Done()
		close(f.ch)
	}()
	return f.ch
}
func (f *fakeSource) Close() error { return nil }

func TestRegistry_OpenAndUnknown(t *testing.T) {
	Register("fake-test", func(u *url.URL) (Source, error) {
		return &fakeSource{name: "fake:" + u.Host + u.Path, ch: make(chan event.Event)}, nil
	})

	s, err := Open("fake-test://example/foo")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !strings.HasPrefix(s.Name(), "fake:") {
		t.Fatalf("name: %q", s.Name())
	}

	if _, err := Open("does-not-exist://x"); err == nil {
		t.Fatal("expected error for unknown scheme")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register("dup-test", func(*url.URL) (Source, error) { return nil, nil })
	Register("dup-test", func(*url.URL) (Source, error) { return nil, nil })
}
