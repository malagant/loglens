// Package k8s implements a k8s:// Source that tails pod logs by shelling out
// to the local kubectl binary. v0.1.0 deliberately avoids the in-tree client
// dependency tree — a native client is on the v0.2 roadmap.
//
// URI form:
//
//	k8s://[<context>/]<namespace>/<pod-or-selector>
//
// `pod-or-selector` is a literal pod name unless it contains an '=' character,
// in which case it is passed to kubectl as a label selector (-l). For selector
// mode, every matching pod is tailed concurrently and the pod list is refreshed
// on a short interval so pods that appear mid-stream are picked up.
//
// All kubectl failures (missing binary, kubeconfig errors, transient API
// errors, pod-disappeared) are surfaced as inline level=error events instead
// of crashing the TUI.
package k8s

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/source"
)

const (
	defaultKubectl     = "kubectl"
	defaultPollPeriod  = 5 * time.Second
	stdoutScannerLimit = 1 << 20 // 1 MiB — generous for stack traces in single lines.
)

func init() {
	source.Register("k8s", func(u *url.URL) (source.Source, error) {
		return parseURI(u)
	})
}

// Source tails one or many pods via `kubectl logs -f`. Construct via the URI
// registry or New; never zero-value.
type Source struct {
	context   string
	namespace string
	target    string // pod name or label selector
	selector  bool

	kubectlPath string
	pollPeriod  time.Duration

	once   sync.Once
	closed chan struct{}
}

// Option configures a Source.
type Option func(*Source)

// WithKubectlPath overrides the kubectl binary path. Tests use this to point
// at a stub script.
func WithKubectlPath(p string) Option { return func(s *Source) { s.kubectlPath = p } }

// WithPollPeriod overrides the selector refresh interval. Lower values pick
// up new pods sooner at the cost of CPU and kubectl calls.
func WithPollPeriod(d time.Duration) Option { return func(s *Source) { s.pollPeriod = d } }

// New constructs a Source directly without going through the URI registry.
// Useful for tests and embedding.
func New(ctxName, namespace, target string, opts ...Option) *Source {
	s := &Source{
		context:     ctxName,
		namespace:   namespace,
		target:      target,
		selector:    strings.Contains(target, "="),
		kubectlPath: defaultKubectl,
		pollPeriod:  defaultPollPeriod,
		closed:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func parseURI(u *url.URL) (*Source, error) {
	if u.Scheme != "k8s" {
		return nil, fmt.Errorf("k8s: not a k8s:// URL: %s", u)
	}
	// url.Parse maps "k8s://A/B/C" → Host=A, Path=/B/C.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	var ctxName, ns, target string
	switch len(parts) {
	case 1:
		ns = u.Host
		target = parts[0]
	case 2:
		ctxName = u.Host
		ns = parts[0]
		target = parts[1]
	default:
		return nil, fmt.Errorf("k8s: too many path segments in %q (expected k8s://[ctx/]ns/pod-or-selector)", u)
	}
	if ns == "" {
		return nil, fmt.Errorf("k8s: namespace is required: %q", u)
	}
	if target == "" {
		return nil, fmt.Errorf("k8s: pod or selector is required: %q", u)
	}
	return New(ctxName, ns, target), nil
}

// Name implements source.Source.
func (s *Source) Name() string {
	return s.uri(s.target)
}

func (s *Source) uri(target string) string {
	var b strings.Builder
	b.WriteString("k8s://")
	if s.context != "" {
		b.WriteString(s.context)
		b.WriteByte('/')
	}
	b.WriteString(s.namespace)
	b.WriteByte('/')
	b.WriteString(target)
	return b.String()
}

// Close implements source.Source. Idempotent.
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Treat Close() like ctx cancellation so child kubectl processes exit.
	go func() {
		select {
		case <-s.closed:
			cancel()
		case <-ctx.Done():
		}
	}()

	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		close(out)
	}()

	if !s.selector {
		s.tailPod(ctx, s.target, out)
		return
	}

	// Selector mode: poll the pod list, spawn a follower per new pod.
	var (
		mu       sync.Mutex
		followed = map[string]struct{}{}
	)
	spawn := func(pod string) {
		mu.Lock()
		if _, ok := followed[pod]; ok {
			mu.Unlock()
			return
		}
		followed[pod] = struct{}{}
		mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.tailPod(ctx, pod, out)
		}()
	}

	refresh := func() {
		pods, err := s.listPods(ctx)
		if err != nil {
			if ctx.Err() == nil {
				s.emit(ctx, out, errorEvent(s.Name(), fmt.Sprintf("kubectl get pods -l %s: %v", s.target, err)))
			}
			return
		}
		for _, p := range pods {
			spawn(p)
		}
	}
	refresh()

	ticker := time.NewTicker(s.pollPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}

func (s *Source) globalFlags() []string {
	args := make([]string, 0, 4)
	if s.context != "" {
		args = append(args, "--context", s.context)
	}
	args = append(args, "-n", s.namespace)
	return args
}

func (s *Source) listPods(ctx context.Context) ([]string, error) {
	args := append(s.globalFlags(), "get", "pods", "-l", s.target, "-o", "name")
	cmd := exec.CommandContext(ctx, s.kubectlPath, args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var pods []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pods = append(pods, strings.TrimPrefix(line, "pod/"))
	}
	return pods, nil
}

// tailPod runs `kubectl logs -f --timestamps --all-containers --prefix <pod>`
// and forwards each line as an event. Errors are emitted as level=error events
// rather than bubbled up — a missing pod or transient API blip should not tear
// down the whole TUI.
func (s *Source) tailPod(ctx context.Context, pod string, out chan<- event.Event) {
	args := append(s.globalFlags(), "logs", "-f", "--timestamps", "--all-containers", "--prefix", pod)
	cmd := exec.CommandContext(ctx, s.kubectlPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.emit(ctx, out, errorEvent(s.uri(pod), fmt.Sprintf("kubectl pipe: %v", err)))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.emit(ctx, out, errorEvent(s.uri(pod), fmt.Sprintf("kubectl pipe: %v", err)))
		return
	}

	if err := cmd.Start(); err != nil {
		s.emit(ctx, out, errorEvent(s.uri(pod), fmt.Sprintf("kubectl start: %v", err)))
		return
	}

	var stderrBuf strings.Builder
	errDone := make(chan struct{})
	go func() {
		defer close(errDone)
		_, _ = io.Copy(stringBuilderWriter{&stderrBuf}, stderr)
	}()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), stdoutScannerLimit)
	for sc.Scan() {
		ev := parseLine(s.uri(pod), sc.Text())
		if !s.emit(ctx, out, ev) {
			break
		}
	}

	<-errDone
	waitErr := cmd.Wait()
	// ctx-cancel kills the process; that path is not an error we should surface.
	if ctx.Err() != nil {
		return
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		s.emit(ctx, out, errorEvent(s.uri(pod), msg))
	}
}

func (s *Source) emit(ctx context.Context, out chan<- event.Event, ev event.Event) bool {
	select {
	case out <- ev:
		return true
	case <-ctx.Done():
		return false
	case <-s.closed:
		return false
	}
}

// parseLine strips the `[pod/X/C]` prefix that `kubectl logs --prefix` emits,
// then a leading RFC3339Nano timestamp from `--timestamps`, then feeds the
// remaining payload through the shared JSON/text parser so JSON pod logs are
// still drilled into fields.
func parseLine(srcName, line string) event.Event {
	rest := line
	if strings.HasPrefix(rest, "[") {
		if end := strings.Index(rest, "] "); end > 0 {
			rest = rest[end+2:]
		}
	}
	ts := time.Time{}
	if i := strings.IndexByte(rest, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, rest[:i]); err == nil {
			ts = t
			rest = rest[i+1:]
		}
	}
	now := time.Now
	if !ts.IsZero() {
		now = func() time.Time { return ts }
	}
	return event.ParseJSONLine(srcName, rest, now)
}

func errorEvent(srcName, msg string) event.Event {
	return event.Event{
		Timestamp: time.Now(),
		Source:    srcName,
		Level:     event.LevelError,
		Raw:       msg,
	}
}

type stringBuilderWriter struct{ b *strings.Builder }

func (w stringBuilderWriter) Write(p []byte) (int, error) {
	if w.b == nil {
		return 0, errors.New("nil builder")
	}
	return w.b.Write(p)
}
