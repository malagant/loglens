# Changelog

All notable changes to LogLens are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
from `v0.1.0` onwards.

## [Unreleased]

## [v0.1.0] — 2026-05-13

First public release. Daily-driver-ready for tailing local files and
Kubernetes pods.

### Added

- **Source pipeline.** Pluggable `source://URI` registry, lock-free merger
  with a ring buffer, and a 16ms-batched pump that protects the TUI from
  bursty producers. (SPA-13)
- **`file://` source.** Tails a single file with `tail -F` semantics —
  survives truncation and inode-swap rotation. (SPA-13)
- **`k8s://` source.** Streams from one pod or every pod matching a label
  selector by shelling out to `kubectl logs -f --timestamps
  --all-containers --prefix`. Inherits whatever auth plugin you already
  have working (oidc, aws-iam-authenticator, gke-gcloud-auth-plugin, …).
  (SPA-25)
- **TUI shell.** Three-pane layout (sources / stream / detail), footer
  keymap, `?` help overlay, and reflow on `WindowSizeMsg`. (SPA-22)
- **Filter DSL.** Slash-prompt language with `level=…`, `source=…`,
  `field.<key>=…`, `/regex-ish substring/`, baretokens, and `!` negation.
  ANDed predicates, live preview as you type, Enter to commit. (SPA-23)
- **JSON drill-down + correlation.** Detail pane parses the selected
  row's JSON into a collapsible tree (`l`/`h`/`j`/`k`, `y` to copy via
  OSC 52). Configurable request-id detector via `--correlate` highlights
  every row that shares the selected request's id with an inline accent.
  (SPA-24)
- **Performance budget + regression guard.** Cold start <100 ms, key
  response <16 ms (~130 M predicate evaluations/s on macOS arm64), idle
  CPU ≈ 0%. CI fails if the matcher throughput on a representative
  compound predicate regresses more than 30% below the recorded
  per-platform baseline. (SPA-26)
- **TUI source wiring.** `--source` is now a first-class, repeatable flag
  that feeds the merged stream straight into the TUI. The pre-v0.1.0
  developer-only dump mode is preserved behind `LOGLENS_DUMP=1` for
  pipeline smoke tests. (SPA-27)
- **Docs + demo.** `docs/demo.gif` (VHS, source in `docs/demo.tape`) and
  `docs/demo.cast` (asciinema v3, source in `docs/cast.sh`), reproducible
  via `make demo`. README quickstart covers install → tail file → filter
  → k8s tail. (SPA-27)

### Distribution

- Multi-platform binaries (linux/macOS/windows × amd64/arm64) via
  GoReleaser, signed keylessly with Sigstore cosign + the GitHub Actions
  workflow identity, and verifiable against the public Rekor transparency
  log.
- Homebrew tap (`loglens/homebrew-tap`) and Scoop bucket
  (`loglens/scoop-bucket`) auto-updated from the release pipeline.

### Notes

- The `kubectl` shell-out is intentional for v0.1: it reuses whatever
  auth you already have working. A native `client-go` source is on the
  v0.2 roadmap.
- The filter grammar is intentionally small. Booleans, regex literals,
  time-window predicates, and aggregations are reserved for v0.2.

[Unreleased]: https://github.com/malagant/loglens/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/malagant/loglens/releases/tag/v0.1.0
