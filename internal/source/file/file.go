// Package file implements a file:// Source that tails a single log file with
// `tail -F` semantics. It survives truncation and rotation (the path is
// re-resolved when the inode changes or the file shrinks), reads line-by-line
// without loading the whole file into memory, and respects ctx cancellation.
//
// fsnotify is used to wake up promptly on writes when the platform supports
// it; a short polling interval is always running as a fallback so the source
// keeps working on filesystems where fsnotify is unreliable (NFS, fuse,
// some container layers).
package file

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/source"
)

const (
	defaultPollInterval = 100 * time.Millisecond
	readBufferSize      = 16 * 1024
)

func init() {
	source.Register("file", func(u *url.URL) (source.Source, error) {
		p, err := source.FilePath(u)
		if err != nil {
			return nil, err
		}
		fromStart := true
		if u.Query().Get("from") == "end" {
			fromStart = false
		}
		return &Source{path: p, pollInterval: defaultPollInterval, fromStart: fromStart}, nil
	})
}

// Source tails a single file.
type Source struct {
	path         string
	pollInterval time.Duration
	fromStart    bool

	once   sync.Once
	closed chan struct{}
}

// New constructs a Source directly (without going through the URI registry).
// Useful for tests and internal callers.
func New(path string, opts ...Option) *Source {
	s := &Source{
		path:         path,
		pollInterval: defaultPollInterval,
		fromStart:    true,
		closed:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures a Source.
type Option func(*Source)

// WithPollInterval overrides the fallback poll interval. Lower values reduce
// latency on filesystems that don't support fsnotify, at the cost of CPU.
func WithPollInterval(d time.Duration) Option {
	return func(s *Source) { s.pollInterval = d }
}

// FromEnd configures the source to begin tailing at the end of the file,
// matching `tail -F` semantics. Default is to read from the start.
func FromEnd() Option { return func(s *Source) { s.fromStart = false } }

// Name implements source.Source.
func (s *Source) Name() string { return "file://" + s.path }

// Close implements source.Source. It is idempotent.
func (s *Source) Close() error {
	s.once.Do(func() {
		if s.closed != nil {
			close(s.closed)
		}
	})
	return nil
}

// Stream implements source.Source.
func (s *Source) Stream(ctx context.Context) <-chan event.Event {
	if s.closed == nil {
		s.closed = make(chan struct{})
	}
	out := make(chan event.Event, 64)
	go s.run(ctx, out)
	return out
}

func (s *Source) run(ctx context.Context, out chan<- event.Event) {
	defer close(out)

	var (
		f       *os.File
		curInfo os.FileInfo
		// leftover holds bytes of a partial line that has not yet seen a '\n'.
		leftover []byte
	)
	closeFile := func() {
		if f != nil {
			_ = f.Close()
			f = nil
		}
		curInfo = nil
		leftover = leftover[:0]
	}
	defer closeFile()

	openAt := func(seekEnd bool) bool {
		closeFile()
		nf, err := os.Open(s.path)
		if err != nil {
			return false
		}
		info, err := nf.Stat()
		if err != nil {
			_ = nf.Close()
			return false
		}
		if seekEnd {
			if _, err := nf.Seek(0, io.SeekEnd); err != nil {
				_ = nf.Close()
				return false
			}
		}
		f = nf
		curInfo = info
		return true
	}

	readAvailable := func() bool {
		if f == nil {
			return true
		}
		buf := make([]byte, readBufferSize)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				data := append(leftover, buf[:n]...)
				leftover = nil
				for {
					idx := bytes.IndexByte(data, '\n')
					if idx < 0 {
						leftover = append(leftover[:0], data...)
						break
					}
					line := string(data[:idx])
					data = data[idx+1:]
					ev := event.ParseJSONLine(s.Name(), line, time.Now)
					select {
					case out <- ev:
					case <-ctx.Done():
						return false
					case <-s.closed:
						return false
					}
				}
			}
			if errors.Is(err, io.EOF) {
				return true
			}
			if err != nil {
				return true
			}
			if n == 0 {
				return true
			}
		}
	}

	// Detect rotation/truncation and (re)open as needed. Returns true if the
	// file is currently open and readable.
	checkRotation := func(initial bool) {
		info, err := os.Stat(s.path)
		if err != nil {
			// File temporarily missing. Drop our handle and wait for it.
			closeFile()
			return
		}
		if curInfo == nil {
			// First open: honor fromStart vs fromEnd.
			openAt(!s.fromStart && initial)
			return
		}
		if !os.SameFile(curInfo, info) {
			// Rotated — new file at the same path. Read it from the start.
			openAt(false)
			return
		}
		// Same file. Check for truncation.
		if f != nil {
			pos, _ := f.Seek(0, io.SeekCurrent)
			if info.Size() < pos {
				_, _ = f.Seek(0, io.SeekStart)
				leftover = leftover[:0]
			}
		}
	}

	checkRotation(true)
	if !readAvailable() {
		return
	}

	// Optional fsnotify on the parent dir for low-latency wakeups. Failures
	// (unsupported FS, permission denied) are non-fatal — the poll loop will
	// still deliver events at pollInterval cadence.
	var fsEvents <-chan fsnotify.Event
	if watcher, werr := fsnotify.NewWatcher(); werr == nil {
		if e := watcher.Add(filepath.Dir(s.path)); e == nil {
			fsEvents = watcher.Events
			defer func() { _ = watcher.Close() }()
		} else {
			_ = watcher.Close()
		}
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		case <-ticker.C:
		case <-fsEvents:
		}
		checkRotation(false)
		if !readAvailable() {
			return
		}
	}
}
