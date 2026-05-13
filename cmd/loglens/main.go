package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loglens/loglens/internal/correlate"
	"github.com/loglens/loglens/internal/pipeline"
	"github.com/loglens/loglens/internal/source"
	_ "github.com/loglens/loglens/internal/source/file" // register file:// scheme
	_ "github.com/loglens/loglens/internal/source/k8s"  // register k8s:// scheme
	"github.com/loglens/loglens/internal/tui"
)

// tBoot is stamped before flag parsing so LOGLENS_PROFILE=1 can report the
// full in-process time from argv handling to Bubble Tea handoff.
var tBoot = time.Now()

// sourceList collects repeated --source flags into an ordered list. Each
// entry is one source URI (see scheme list in `loglens --help`).
type sourceList []string

func (s *sourceList) String() string     { return strings.Join(*s, ",") }
func (s *sourceList) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")

	var sources sourceList
	flag.Var(&sources, "source",
		"source URI to tail; repeatable. Schemes: file://PATH, k8s://[ctx/]ns/<pod|selector>")

	var correlateKeys string
	flag.StringVar(&correlateKeys, "correlate", "",
		"comma-separated JSON keys to use for request-id correlation "+
			"(default: request_id,trace_id,x-request-id,requestId)")
	flag.Parse()

	profile := os.Getenv("LOGLENS_PROFILE") == "1"

	if *versionFlag {
		fmt.Println(tui.Version)
		if profile {
			bootLog("version", time.Since(tBoot))
		}
		return
	}

	// LOGLENS_DUMP=1 is the SPA-13 source-pipeline smoke path: connect every
	// --source to the merger and stream raw lines to stdout. Keeps the
	// pre-TUI surface accessible from `time loglens --source ...` benches
	// without leaving footprints in the user-facing CLI.
	if os.Getenv("LOGLENS_DUMP") == "1" && len(sources) > 0 {
		if profile {
			bootLog("source-dump-ready", time.Since(tBoot))
		}
		runSourceDump(sources)
		return
	}

	if profile {
		bootLog("tui-ready", time.Since(tBoot))
	}
	// LOGLENS_PROFILE_EXIT=1 exits right before the event loop so that
	// `time loglens` measures end-to-end cold start without waiting for input.
	if os.Getenv("LOGLENS_PROFILE_EXIT") == "1" {
		return
	}

	tuiModel := tui.New()
	if correlateKeys != "" {
		tuiModel = tuiModel.WithDetector(correlate.Parse(correlateKeys))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Open every source up-front so any misconfiguration surfaces as a
	// terminal error instead of silently disappearing into the TUI's empty
	// state.
	var opened []source.Source
	for _, uri := range sources {
		src, err := source.Open(uri)
		if err != nil {
			fmt.Fprintf(os.Stderr, "loglens: %s: %v\n", uri, err)
			for _, prev := range opened {
				_ = prev.Close()
			}
			os.Exit(2)
		}
		opened = append(opened, src)
	}

	var merger *pipeline.Merger
	if len(opened) > 0 {
		merger = pipeline.New(4096)
		for _, src := range opened {
			merger.Add(ctx, src.Stream(ctx))
		}
	}

	p := tea.NewProgram(tuiModel, tea.WithAltScreen())

	if merger != nil {
		go tui.PumpEvents(ctx, p, merger.Events())
	}

	_, runErr := p.Run()

	cancel()
	if merger != nil {
		merger.Close()
	}
	for _, src := range opened {
		_ = src.Close()
	}

	if runErr != nil {
		fmt.Fprintln(os.Stderr, "loglens:", runErr)
		os.Exit(1)
	}
}

// bootLog writes one structured line to stderr:
//
//	loglens: boot stage=<s> elapsed_ms=<f>
//
// Stderr keeps it out of pipelines; microsecond resolution keeps it readable.
func bootLog(stage string, elapsed time.Duration) {
	fmt.Fprintf(os.Stderr, "loglens: boot stage=%s elapsed_ms=%.3f\n",
		stage, float64(elapsed.Microseconds())/1000.0)
}

// runSourceDump connects every source to the merger and writes each event's
// raw line to stdout. It exits when the user sends SIGINT or every source
// closes. Used by the SPA-13 pipeline smoke (`LOGLENS_DUMP=1 loglens ...`).
func runSourceDump(uris []string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	m := pipeline.New(4096)
	defer m.Close()

	var opened []source.Source
	for _, uri := range uris {
		src, err := source.Open(uri)
		if err != nil {
			fmt.Fprintf(os.Stderr, "loglens: %s: %v\n", uri, err)
			for _, prev := range opened {
				_ = prev.Close()
			}
			os.Exit(2)
		}
		opened = append(opened, src)
		m.Add(ctx, src.Stream(ctx))
	}
	defer func() {
		for _, src := range opened {
			_ = src.Close()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-m.Events():
			if !ok {
				return
			}
			fmt.Println(ev.Raw)
		}
	}
}
