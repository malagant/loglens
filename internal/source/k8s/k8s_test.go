//go:build !windows

package k8s_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/source"
	k8ssrc "github.com/loglens/loglens/internal/source/k8s"
)

// stubPath returns the absolute path to the kubectl stub script. Resolved once
// at package init so test cases don't all have to repeat the dance.
func stubPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/kubectl-stub.sh")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("stub missing: %v", err)
	}
	return abs
}

// drain reads up to want events with a deadline. Returns whatever it has when
// the deadline is hit so the caller can show a useful diff.
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

// scenario writes per-pod log fixtures and returns a configured env slice. The
// stub reads STUB_PODS_FILE / STUB_LOGS_DIR from its own environment, so each
// test sets these via t.Setenv.
func writeLogs(t *testing.T, dir string, logs map[string]string) {
	t.Helper()
	for pod, content := range logs {
		if err := os.WriteFile(filepath.Join(dir, pod), []byte(content), 0o644); err != nil {
			t.Fatalf("write log fixture for %s: %v", pod, err)
		}
	}
}

func writePodList(t *testing.T, dir string, pods []string) string {
	t.Helper()
	path := filepath.Join(dir, "pods.txt")
	if err := os.WriteFile(path, []byte(strings.Join(pods, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write pod list: %v", err)
	}
	return path
}

func TestK8sSource_LiteralPodTails(t *testing.T) {
	dir := t.TempDir()
	writeLogs(t, dir, map[string]string{
		"api-7d": "[pod/api-7d/web] 2026-05-13T10:00:00.000Z hello\n" +
			"[pod/api-7d/web] 2026-05-13T10:00:01.500Z world\n",
	})
	t.Setenv("STUB_LOGS_DIR", dir)

	s := k8ssrc.New("", "default", "api-7d", k8ssrc.WithKubectlPath(stubPath(t)))
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ch := s.Stream(ctx)

	got := drain(t, ch, 2, 2*time.Second)
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d: %+v", len(got), got)
	}
	if got[0].Raw != "hello" || got[1].Raw != "world" {
		t.Fatalf("payloads: %q / %q", got[0].Raw, got[1].Raw)
	}
	wantTS, _ := time.Parse(time.RFC3339Nano, "2026-05-13T10:00:00.000Z")
	if !got[0].Timestamp.Equal(wantTS) {
		t.Fatalf("timestamp not parsed from kubectl --timestamps: %v", got[0].Timestamp)
	}
	if !strings.HasSuffix(got[0].Source, "/api-7d") {
		t.Fatalf("event source should include pod name, got %q", got[0].Source)
	}
}

func TestK8sSource_SelectorTailsAllPods(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.Mkdir(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogs(t, logsDir, map[string]string{
		"api-1": "[pod/api-1/web] 2026-05-13T10:00:00Z a1\n[pod/api-1/web] 2026-05-13T10:00:01Z a2\n",
		"api-2": "[pod/api-2/web] 2026-05-13T10:00:00Z b1\n[pod/api-2/web] 2026-05-13T10:00:01Z b2\n",
	})
	t.Setenv("STUB_PODS_FILE", writePodList(t, dir, []string{"api-1", "api-2"}))
	t.Setenv("STUB_LOGS_DIR", logsDir)

	s := k8ssrc.New("", "default", "app=api",
		k8ssrc.WithKubectlPath(stubPath(t)),
		k8ssrc.WithPollPeriod(50*time.Millisecond),
	)
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ch := s.Stream(ctx)

	got := drain(t, ch, 4, 2*time.Second)
	if len(got) != 4 {
		t.Fatalf("want 4 events, got %d: %+v", len(got), got)
	}

	raws := make([]string, 0, 4)
	sources := map[string]struct{}{}
	for _, ev := range got {
		raws = append(raws, ev.Raw)
		sources[ev.Source] = struct{}{}
	}
	sort.Strings(raws)
	wantRaws := []string{"a1", "a2", "b1", "b2"}
	if strings.Join(raws, ",") != strings.Join(wantRaws, ",") {
		t.Fatalf("payloads: got %v want %v", raws, wantRaws)
	}
	if len(sources) != 2 {
		t.Fatalf("expected two distinct per-pod sources, got %d: %v", len(sources), sources)
	}
}

func TestK8sSource_SelectorPicksUpNewPodMidStream(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.Mkdir(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogs(t, logsDir, map[string]string{
		"api-1": "[pod/api-1/web] 2026-05-13T10:00:00Z first\n",
		"api-2": "[pod/api-2/web] 2026-05-13T10:00:05Z second\n",
	})
	podListPath := writePodList(t, dir, []string{"api-1"})
	t.Setenv("STUB_PODS_FILE", podListPath)
	t.Setenv("STUB_LOGS_DIR", logsDir)

	s := k8ssrc.New("", "default", "app=api",
		k8ssrc.WithKubectlPath(stubPath(t)),
		k8ssrc.WithPollPeriod(50*time.Millisecond),
	)
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ch := s.Stream(ctx)

	if first := drain(t, ch, 1, 2*time.Second); len(first) != 1 || first[0].Raw != "first" {
		t.Fatalf("first event: %+v", first)
	}

	// Now add api-2 to the selector match set. The poll loop should spawn a
	// new follower for it without restarting the source.
	if err := os.WriteFile(podListPath, []byte("api-1\napi-2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rest := drain(t, ch, 1, 2*time.Second)
	if len(rest) != 1 || rest[0].Raw != "second" {
		t.Fatalf("new-pod event: %+v", rest)
	}
}

func TestK8sSource_KubectlErrorBecomesInlineErrorEvent(t *testing.T) {
	t.Setenv("STUB_LOGS_FAIL", "Error from server (NotFound): pods \"missing\" not found")

	s := k8ssrc.New("", "default", "missing", k8ssrc.WithKubectlPath(stubPath(t)))
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ch := s.Stream(ctx)

	got := drain(t, ch, 1, 2*time.Second)
	if len(got) != 1 {
		t.Fatalf("want 1 error event, got %d: %+v", len(got), got)
	}
	if got[0].Level != event.LevelError {
		t.Fatalf("level: %q want error", got[0].Level)
	}
	if !strings.Contains(got[0].Raw, "NotFound") {
		t.Fatalf("error body should include stderr, got %q", got[0].Raw)
	}
}

func TestK8sSource_SelectorGetPodsErrorBecomesInlineErrorEvent(t *testing.T) {
	t.Setenv("STUB_GET_FAIL", "error: You must be logged in to the server")

	s := k8ssrc.New("", "default", "app=api",
		k8ssrc.WithKubectlPath(stubPath(t)),
		k8ssrc.WithPollPeriod(50*time.Millisecond),
	)
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch := s.Stream(ctx)

	got := drain(t, ch, 1, 1500*time.Millisecond)
	if len(got) == 0 {
		t.Fatal("expected at least one error event surfaced from kubectl get pods")
	}
	if got[0].Level != event.LevelError {
		t.Fatalf("level: %q want error", got[0].Level)
	}
	if !strings.Contains(got[0].Raw, "logged in") {
		t.Fatalf("error body should include stderr, got %q", got[0].Raw)
	}
}

func TestK8sSource_URIRegistration(t *testing.T) {
	cases := []struct {
		uri      string
		wantName string
	}{
		{"k8s://default/api-7d", "k8s://default/api-7d"},
		{"k8s://prod-ctx/default/api-7d", "k8s://prod-ctx/default/api-7d"},
		{"k8s://default/app=api", "k8s://default/app=api"},
	}
	for _, tc := range cases {
		t.Run(tc.uri, func(t *testing.T) {
			s, err := source.Open(tc.uri)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer s.Close()
			if s.Name() != tc.wantName {
				t.Fatalf("name: got %q want %q", s.Name(), tc.wantName)
			}
		})
	}
}

func TestK8sSource_URIRejectsBadShapes(t *testing.T) {
	bad := []string{
		"k8s:///nopod",                // missing namespace
		"k8s://default/",              // missing pod
		"k8s://ctx/ns/extra/pod-name", // too many segments
	}
	for _, uri := range bad {
		t.Run(uri, func(t *testing.T) {
			if _, err := source.Open(uri); err == nil {
				t.Fatalf("expected error for %q", uri)
			}
		})
	}
}
