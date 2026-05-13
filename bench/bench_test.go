// Package bench houses LogLens performance benchmarks.
// Two scenarios are covered:
//
//   - BenchmarkMatcher* — filter.Query.Match throughput on the v0.1 DSL.
//   - BenchmarkMergerThroughput — end-to-end Merger throughput from a single producer.
//
// Run locally: go test -bench . -benchmem ./bench/...
package bench

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/loglens/loglens/internal/event"
	"github.com/loglens/loglens/internal/filter"
	"github.com/loglens/loglens/internal/pipeline"
)

// matcherCorpus returns a deterministic slab of events shaped like real JSON
// log lines. Level and source use coprime strides so the compound predicate
// (level=error AND source=k8s) has a non-zero hit rate and the matcher must
// evaluate all predicates.
func matcherCorpus(n int) []event.Event {
	levels := []event.Level{event.LevelInfo, event.LevelWarn, event.LevelError, event.LevelDebug}
	sources := []string{"file:///var/log/app.log", "k8s://default/api-7d/api", "docker://nginx", "journald:///"}
	out := make([]event.Event, n)
	for i := 0; i < n; i++ {
		lvl := levels[i%len(levels)]
		src := sources[(i/len(levels))%len(sources)]
		out[i] = event.Event{
			Timestamp: time.Unix(int64(1_700_000_000+i), 0),
			Source:    src,
			Level:     lvl,
			Fields: map[string]any{
				"request_id": "req-" + strconv.Itoa(i%997),
				"user_id":    strconv.Itoa(i % 53),
				"msg":        "handled request from upstream service in 12ms",
			},
			Raw: `{"level":"` + string(lvl) +
				`","source":"` + src +
				`","msg":"handled request from upstream service in 12ms","request_id":"req-` +
				strconv.Itoa(i%997) + `"}`,
		}
	}
	return out
}

func benchmarkMatcher(b *testing.B, query string) {
	b.Helper()
	q, err := filter.Parse(query)
	if err != nil {
		b.Fatalf("parse %q: %v", query, err)
	}
	corpus := matcherCorpus(4096)
	b.ReportAllocs()
	b.ResetTimer()
	var matched int
	for i := 0; i < b.N; i++ {
		if q.Match(corpus[i%len(corpus)]) {
			matched++
		}
	}
	b.ReportMetric(float64(matched)/float64(b.N), "matched/op")
}

func BenchmarkMatcherEmpty(b *testing.B)    { benchmarkMatcher(b, "") }
func BenchmarkMatcherLevel(b *testing.B)    { benchmarkMatcher(b, "level=error") }
func BenchmarkMatcherSubstr(b *testing.B)   { benchmarkMatcher(b, "upstream") }
func BenchmarkMatcherField(b *testing.B)    { benchmarkMatcher(b, "field.user_id=42") }
func BenchmarkMatcherCompound(b *testing.B) { benchmarkMatcher(b, "level=error source=k8s /upstream service/") }

// BenchmarkMergerThroughput measures sustained events/sec through the Merger
// from a single producer into a consumer that drains as fast as it can.
func BenchmarkMergerThroughput(b *testing.B) {
	m := pipeline.New(4096)
	defer m.Close()

	in := make(chan event.Event, 1024)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Add(ctx, in)

	ev := event.Event{Source: "file:///var/log/app.log", Raw: "synthetic event"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range m.Events() {
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in <- ev
	}
	close(in)
	b.StopTimer()
	m.Close()
	<-done
}
