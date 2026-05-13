//go:build !windows

package smoke_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// buildBinary compiles the loglens binary into a temp dir and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "loglens")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/loglens")
	build.Env = append(build.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// ptySession wraps the pty machinery shared by both smoke tests.
type ptySession struct {
	ptmx *os.File
	cmd  *exec.Cmd
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan struct{}
}

// startPTY launches bin in a pty with terminal-capability auto-response.
// It sets an explicit window size so Bubble Tea receives a WindowSizeMsg and
// renders the multi-pane shell instead of the zero-size placeholder.
func startPTY(t *testing.T, bin string, args ...string) *ptySession {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(cmd.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 120})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	s := &ptySession{ptmx: ptmx, cmd: cmd, done: make(chan struct{})}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				s.mu.Lock()
				s.buf.Write(chunk)
				s.mu.Unlock()
				// Respond to Bubble Tea terminal probes so rendering proceeds.
				if bytes.Contains(chunk, []byte("\x1b[6n")) {
					_, _ = ptmx.Write([]byte("\x1b[1;1R"))
				}
				if bytes.Contains(chunk, []byte("\x1b]11;?")) {
					_, _ = ptmx.Write([]byte("\x1b]11;rgb:0000/0000/0000\x07"))
				}
			}
			if err != nil {
				close(s.done)
				return
			}
		}
	}()
	return s
}

func (s *ptySession) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *ptySession) waitFor(t *testing.T, needle string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.output(), needle) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in output:\n%s", needle, s.output())
}

func (s *ptySession) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := io.WriteString(s.ptmx, keys); err != nil {
		t.Fatalf("write to pty: %v", err)
	}
}

func (s *ptySession) waitExit(t *testing.T, timeout time.Duration) {
	t.Helper()
	exitC := make(chan error, 1)
	go func() { exitC <- s.cmd.Wait() }()
	select {
	case err := <-exitC:
		if err != nil {
			t.Fatalf("loglens exited with error: %v\noutput:\n%s", err, s.output())
		}
	case <-time.After(timeout):
		_ = s.cmd.Process.Kill()
		t.Fatalf("loglens did not exit within %s\noutput:\n%s", timeout, s.output())
	}
	<-s.done
}

// TestTUIBootsInPTY verifies the TUI renders the multi-pane shell (with
// "LogLens" branding in the source pane title) and exits cleanly on `q`.
func TestTUIBootsInPTY(t *testing.T) {
	if testing.Short() {
		t.Skip("pty smoke skipped in -short mode")
	}
	bin := buildBinary(t)
	s := startPTY(t, bin)
	defer func() { _ = s.ptmx.Close() }()

	// Wait for the multi-pane render: the source pane title includes "LogLens".
	s.waitFor(t, "LogLens", 5*time.Second)

	// The keymap footer must be visible (basic bindings like "quit").
	s.waitFor(t, "quit", 3*time.Second)

	s.send(t, "q")
	s.waitExit(t, 5*time.Second)
}

// TestHelpOverlayOpensAndCloses verifies that pressing `?` shows the help
// overlay (listing all bindings), and `?` again dismisses it.
func TestHelpOverlayOpensAndCloses(t *testing.T) {
	if testing.Short() {
		t.Skip("pty smoke skipped in -short mode")
	}
	bin := buildBinary(t)
	s := startPTY(t, bin)
	defer func() { _ = s.ptmx.Close() }()

	// Wait for base shell.
	s.waitFor(t, "LogLens", 5*time.Second)

	// Open the help overlay.
	s.send(t, "?")
	s.waitFor(t, "Basic", 3*time.Second)
	s.waitFor(t, "Power user", 3*time.Second)

	// Dismiss with ? again; wait for a string that only appears when the
	// overlay is gone (the stream pane border title, not the help overlay).
	s.send(t, "?")
	s.waitFor(t, "Stream", 3*time.Second)

	s.send(t, "q")
	s.waitExit(t, 5*time.Second)
}
