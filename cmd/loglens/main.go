package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
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

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	// --source is intentionally undocumented in --help; it is a developer
	// escape hatch for verifying the source pipeline end-to-end while the UI
	// is still scaffolding. Real source selection lands when the views ship.
	var sourceURI string
	flag.StringVar(&sourceURI, "source", "", "")
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

	if sourceURI != "" {
		if profile {
			bootLog("source-dump-ready", time.Since(tBoot))
		}
		runSourceDump(sourceURI)
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
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "loglens:", err)
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

// runSourceDump connects a single source to the merger and writes each event's
// raw line to stdout. It exits when the user sends SIGINT or the source closes.
// This is the smallest viable smoke test for the SPA-13 pipeline.
func runSourceDump(uri string) {
	src, err := source.Open(uri)
	if err != nil {
		fmt.Fprintln(os.Stderr, "loglens:", err)
		os.Exit(2)
	}
	defer src.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	m := pipeline.New(4096)
	defer m.Close()
	m.Add(ctx, src.Stream(ctx))

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
