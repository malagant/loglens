// Package source defines the Source interface every LogLens log producer
// implements, plus a registry and URI parser used to instantiate sources from
// the command line. Concrete sources live in sub-packages (e.g. source/file).
package source

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/loglens/loglens/internal/event"
)

// Source produces a stream of normalized events until its context is cancelled
// or Close is called. Stream may be called exactly once per Source; subsequent
// calls return a nil channel. Implementations must close the returned channel
// when they have no more events to deliver.
type Source interface {
	// Name is a stable identifier suitable for the Event.Source field.
	Name() string

	// Stream begins reading and returns a channel of events. The channel is
	// closed when the source ends (context cancellation, file removed, etc.).
	Stream(ctx context.Context) <-chan event.Event

	// Close releases resources. Safe to call multiple times.
	Close() error
}

// Factory builds a Source from a URI. The URI's scheme has already been
// matched by the registry; the factory is responsible for validating the rest.
type Factory func(u *url.URL) (Source, error)

var (
	regMu     sync.RWMutex
	factories = map[string]Factory{}
)

// Register installs a Factory for the given URI scheme. Registering the same
// scheme twice panics — this is a programming error, not a runtime condition.
func Register(scheme string, f Factory) {
	if scheme == "" || f == nil {
		panic("source.Register: scheme and factory required")
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := factories[scheme]; exists {
		panic("source.Register: duplicate scheme " + scheme)
	}
	factories[scheme] = f
}

// Schemes returns the registered scheme names in sorted order. Useful for
// surfacing help text.
func Schemes() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(factories))
	for k := range factories {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Open parses a URI and returns a Source built by the registered factory.
// Bare paths (no scheme) are treated as file:// for ergonomics on the CLI.
func Open(uri string) (Source, error) {
	u, err := ParseURI(uri)
	if err != nil {
		return nil, err
	}
	regMu.RLock()
	f, ok := factories[u.Scheme]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("source: unknown scheme %q (registered: %v)", u.Scheme, Schemes())
	}
	return f(u)
}

// ParseURI normalizes a user-supplied source URI. It accepts:
//   - bare paths      → file:///abs/path or file://./relative
//   - file://path     → file scheme (absolute or relative)
//   - scheme://rest   → as-is
//
// The returned *url.URL always has a non-empty Scheme.
func ParseURI(uri string) (*url.URL, error) {
	if uri == "" {
		return nil, fmt.Errorf("source: empty URI")
	}
	if !strings.Contains(uri, "://") {
		// Bare path. Make it explicit so the file factory can treat it uniformly.
		uri = "file://" + uri
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("source: parse %q: %w", uri, err)
	}
	if u.Scheme == "" {
		return nil, fmt.Errorf("source: missing scheme in %q", uri)
	}
	return u, nil
}

// FilePath extracts a filesystem path from a parsed file:// URL. It accepts
// the common forms produced by ParseURI and returns a cleaned path (no
// trailing slash, no embedded "."), preserving absolute vs. relative form.
func FilePath(u *url.URL) (string, error) {
	if u.Scheme != "file" {
		return "", fmt.Errorf("source: not a file:// URL: %s", u.String())
	}
	// url.Parse puts the first path segment into Host for "file://foo/bar".
	// Reconstruct: if Host is empty (file:///abs) we keep Path; otherwise we
	// re-join Host + Path so relative paths are preserved.
	p := u.Path
	if u.Host != "" && u.Host != "localhost" {
		if p == "" {
			p = u.Host
		} else {
			p = u.Host + p
		}
	}
	if p == "" {
		return "", fmt.Errorf("source: file:// URL has no path: %s", u.String())
	}
	return filepath.Clean(p), nil
}
