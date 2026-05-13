package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loglens/loglens/internal/filter"
)

// TestMatcherRegressionGuard fails the build if BenchmarkMatcherCompound
// throughput drops more than 30% below the recorded per-platform baseline.
// Skip with LOGLENS_SKIP_PERF=1 or -short on noisy machines.
// Platforms without a baseline entry are skipped gracefully.
func TestMatcherRegressionGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping perf regression guard")
	}
	if os.Getenv("LOGLENS_SKIP_PERF") == "1" {
		t.Skip("LOGLENS_SKIP_PERF=1: skipping perf regression guard")
	}

	base := loadBaseline(t)
	want, ok := base.MatcherCompoundOpsPerSec[platformKey()]
	if !ok {
		t.Skipf("no baseline for platform %q; record one in baseline.json to enable the guard", platformKey())
	}

	res := testing.Benchmark(func(b *testing.B) {
		benchmarkMatcher(b, "level=error source=k8s /upstream service/")
	})
	if res.N == 0 || res.NsPerOp() == 0 {
		t.Fatalf("benchmark produced no samples")
	}
	got := 1e9 / float64(res.NsPerOp())

	const allowedDrop = 0.30
	minAllowed := want * (1 - allowedDrop)
	t.Logf("matcher compound: got=%.0f ops/sec  baseline=%.0f ops/sec  floor=%.0f ops/sec  platform=%s",
		got, want, minAllowed, platformKey())
	if got < minAllowed {
		t.Fatalf("matcher throughput regression: got %.0f ops/sec, need >= %.0f ops/sec (%.0f%% drop allowed from baseline %.0f)",
			got, minAllowed, allowedDrop*100, want)
	}

	// Sanity: compound query must match a non-trivial minority of the corpus.
	q, _ := filter.Parse("level=error source=k8s /upstream service/")
	corpus := matcherCorpus(4096)
	matched := 0
	for _, ev := range corpus {
		if q.Match(ev) {
			matched++
		}
	}
	if matched == 0 {
		t.Fatal("compound query matched zero events — corpus or matcher is broken")
	}
	if frac := float64(matched) / float64(len(corpus)); frac > 0.5 {
		t.Fatalf("compound query matched %.0f%% of corpus; expected a minority", frac*100)
	}
}

type baseline struct {
	MatcherCompoundOpsPerSec map[string]float64 `json:"matcherCompoundOpsPerSec"`
	Notes                    string             `json:"notes"`
}

func loadBaseline(t *testing.T) baseline {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("baseline.json"))
	if err != nil {
		t.Fatalf("read baseline.json: %v", err)
	}
	var b baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("parse baseline.json: %v", err)
	}
	return b
}

func platformKey() string { return runtime.GOOS + "/" + runtime.GOARCH }
