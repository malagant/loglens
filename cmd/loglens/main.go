package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loglens/loglens/internal/correlate"
	"github.com/loglens/loglens/internal/pipeline"
	"github.com/loglens/loglens/internal/source"
	_ "github.com/loglens/loglens/internal/source/file" // register file:// scheme
	_ "github.com/loglens/loglens/internal/source/k8s"  // register k8s:// scheme
	"github.com/loglens/loglens/internal/tui"
)

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

	if *versionFlag {
		fmt.Println(tui.Version)
		return
	}

	if correlateKeys != "" {
		tui.SetDetector(correlate.Parse(correlateKeys))
	}

	if sourceURI != "" {
		runSourceDump(sourceURI)
		return
	}

	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "loglens:", err)
		os.Exit(1)
	}
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
