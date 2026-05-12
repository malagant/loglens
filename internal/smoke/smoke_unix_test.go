//go:build !windows

package smoke_test

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestTUIBootsInPTY builds the loglens binary, runs it inside a pseudo-terminal,
// asserts the branding is rendered, sends a quit keystroke, and checks for a
// clean exit. This guards the foundational "the TUI actually boots" invariant
// against accidental regressions in the model or main entry point.
func TestTUIBootsInPTY(t *testing.T) {
	if testing.Short() {
		t.Skip("pty smoke skipped in -short mode")
	}

	binDir := t.TempDir()
	bin := filepath.Join(binDir, "loglens")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	build := exec.Command("go", "build", "-o", bin, "../../cmd/loglens")
	build.Env = append(build.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(bin)
	cmd.Env = append(cmd.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	var out bytes.Buffer
	var mu sync.Mutex
	done := make(chan struct{})
	// Bubble Tea probes the terminal for capabilities (cursor position via DSR
	// `ESC[6n`, background color via OSC 11). A real terminal answers these
	// instantly; a raw PTY does not, so the program blocks before drawing. We
	// answer with sensible defaults so rendering proceeds.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				mu.Lock()
				out.Write(chunk)
				mu.Unlock()
				if bytes.Contains(chunk, []byte("\x1b[6n")) {
					_, _ = ptmx.Write([]byte("\x1b[1;1R"))
				}
				if bytes.Contains(chunk, []byte("\x1b]11;?")) {
					_, _ = ptmx.Write([]byte("\x1b]11;rgb:0000/0000/0000\x07"))
				}
			}
			if err != nil {
				close(done)
				return
			}
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := out.String()
		mu.Unlock()
		if strings.Contains(s, "LogLens") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	rendered := out.String()
	mu.Unlock()
	if !strings.Contains(rendered, "LogLens") {
		t.Fatalf("branding not rendered within deadline; got:\n%s", rendered)
	}

	if _, err := io.WriteString(ptmx, "q"); err != nil {
		t.Fatalf("write quit: %v", err)
	}

	exitC := make(chan error, 1)
	go func() { exitC <- cmd.Wait() }()

	select {
	case err := <-exitC:
		if err != nil {
			t.Fatalf("loglens exited with error: %v\noutput:\n%s", err, out.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("loglens did not exit after q within 5s\noutput:\n%s", out.String())
	}

	<-done
}
