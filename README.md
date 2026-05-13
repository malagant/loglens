# LogLens

One TUI, every log source, one query language.

LogLens is a fast, local-only terminal UI for tailing and searching logs across Kubernetes, local files, journald, and Docker — with a single filter DSL, JSON drill-down, and request-id correlation. No phone-home, no backend, just your terminal.

> Status: `v0.1.0` is the first tagged release suitable for daily use on
> local files and Kubernetes pods. See the roadmap in
> [SPA-6](https://github.com/malagant/loglens/issues) and CHANGELOG.md.

## Demo

![LogLens demo — tail a file, filter for errors, drill into a request](docs/demo.gif)

GIF too heavy? The plaintext fallback is [`docs/demo.cast`](docs/demo.cast)
(`asciinema play docs/demo.cast`). The source-of-truth scripts that produce
both — [`docs/demo.tape`](docs/demo.tape) for the GIF, [`docs/cast.sh`](docs/cast.sh)
for the cast — live in the repo and re-render via `make demo`.

## Install

```sh
# macOS / Linux (Homebrew tap)
brew install malagant/tap/loglens

# Windows (Scoop bucket)
scoop bucket add malagant https://github.com/malagant/scoop-bucket
scoop install loglens

# Container (Linux amd64 / arm64, distroless)
docker run --rm -it ghcr.io/malagant/loglens:latest --help

# Any platform with Go 1.23+
go install github.com/loglens/loglens/cmd/loglens@latest

# Verify
loglens --version
```

Pre-built archives for `linux`, `darwin`, `windows` × `amd64`, `arm64` are
attached to every GitHub Release together with a SHA-256 checksums file and a
keyless cosign signature (see [Verifying release artifacts](#verifying-release-artifacts)).

## Quickstart

LogLens reads from one or more `--source` URIs and merges them into a single
scroll-back. Three steps to get from install to live tail:

```sh
# 1. Tail a local file (try the bundled sample first)
loglens --source file://docs/sample.log

# 2. Open the filter prompt with `/`, then type a predicate and press Enter:
#       level=error
#    The stream narrows live as you type. Esc clears, q quits.

# 3. Swap to a Kubernetes pod (requires kubectl on PATH)
loglens --source k8s://default/api-7d8c9b6f4-x2k9z
```

Press `?` at any time for the full keymap. The interactive flow is in
[`docs/demo.gif`](docs/demo.gif); the keystrokes that produced it live in
[`docs/demo.tape`](docs/demo.tape).

### Source URIs

LogLens reads from one or more **source URIs**. Schemes available in v0.1:

| Scheme  | Form                                          | Notes                                                                              |
| ------- | --------------------------------------------- | ---------------------------------------------------------------------------------- |
| `file://` | `file:///var/log/app.log` or bare path      | Tails a single file with `tail -F` rotation/truncation semantics.                  |
| `k8s://`  | `k8s://[<context>/]<namespace>/<pod-or-selector>` | Shells out to `kubectl logs -f --timestamps --all-containers --prefix`. Requires `kubectl` on `PATH` and a configured kubeconfig. |

Pass `--source` multiple times to merge several streams:

```sh
loglens \
  --source file:///var/log/app.log \
  --source k8s://default/api-7d8c9b6f4-x2k9z \
  --source k8s://default/app=api
```

### Kubernetes examples

```sh
# Tail a single pod in the current kubeconfig context
loglens --source k8s://default/api-7d8c9b6f4-x2k9z

# Tail every pod matching a label selector — new pods are picked up mid-stream
loglens --source k8s://default/app=api

# Pin a non-default context
loglens --source k8s://prod-eu/payments/checkout-worker
```

The `kubectl` dependency is intentional for v0.1.0: it inherits whatever
authentication you already have working (oidc plugins, aws-iam-authenticator,
gke-gcloud-auth-plugin, etc.) without bundling them. A native client-go
implementation is on the v0.2 roadmap. Errors from `kubectl` (missing
binary, bad kubeconfig, pod not found, transient API blips) surface as inline
`level=error` events rather than crashing the TUI.

## Why

`stern` is Kubernetes-only. CloudWatch only opens in a browser. Datadog costs $200/host. `lnav` is brilliant but file-only. Every DevOps engineer ends up juggling four log tools a day. LogLens is the unifier — one keymap, one filter language, every source you own.

## Filter language (v0.1)

LogLens ships a small slash-prompt filter that runs against every event in the
merged stream. Press `/` to open it, type your query, then `Enter` to commit or
`Esc` to cancel. The matcher updates live as you type.

The v0.1 grammar is a whitespace-separated list of predicates, ANDed together:

| Token              | Meaning                                                       |
| ------------------ | ------------------------------------------------------------- |
| `level=error`      | Normalized level match (`trace` `debug` `info` `warn` `error` `fatal`). |
| `source=k8s://ns`  | Substring match against the event source URI.                 |
| `field.user_id=42` | Substring match against the parsed JSON field with that key.  |
| `/needle/`         | Substring match on the raw line; spaces inside `/…/` are kept. |
| `bareword`         | Substring match on the raw line.                              |
| `!<predicate>`     | Negate any predicate above.                                   |

Examples:

```text
level=error                       # only errors
level=warn source=k8s             # warnings from any kubernetes source
/request id=abc/                  # substring with spaces
field.user_id=42 !source=staging  # field match, excluding staging
```

The full DSL (booleans, regex, ranges, time windows) is reserved for v0.2.

## Performance

LogLens is built to the following hard budgets — measured, not aspirational:

| Metric | Budget | Measured (macOS arm64) | Measured (linux amd64) |
| --- | --- | --- | --- |
| Cold start (process exec → TUI ready) | < 100 ms | ~16 ms | _CI pending_ |
| Key-response (filter match, worst case) | < 16 ms | < 0.01 ms (130 M ops/s) | _CI pending_ |
| Idle CPU (blocked on input, no log stream) | ≈ 0% | 0.0% | _CI pending_ |

All three metrics have a regression guard in CI. The matcher guard fails the build if throughput on the compound predicate `level=error source=k8s /upstream/` drops more than 30% below the recorded per-platform baseline (`bench/baseline.json`).

### Measuring locally

```sh
# Cold start
LOGLENS_PROFILE=1 LOGLENS_PROFILE_EXIT=1 loglens
# → loglens: boot stage=tui-ready elapsed_ms=X.XXX

# Matcher throughput
go test -run '^$' -bench BenchmarkMatcherCompound -benchtime 1s ./bench/

# Idle CPU (pass PID of a running loglens process, or omit to auto-detect)
./scripts/idle-cpu.sh [pid]
```

## Verifying release artifacts

Releases are signed with [Sigstore](https://www.sigstore.dev/) **keyless cosign**: the signing identity is the GitHub Actions workflow itself, not a long-lived keypair. To verify the checksums file for a release:

```sh
VERSION=v0.1.0
BASE="https://github.com/malagant/loglens/releases/download/${VERSION}"
curl -sLO "${BASE}/loglens_${VERSION#v}_checksums.txt"
curl -sLO "${BASE}/loglens_${VERSION#v}_checksums.txt.sig"
curl -sLO "${BASE}/loglens_${VERSION#v}_checksums.txt.pem"

cosign verify-blob \
  --certificate "loglens_${VERSION#v}_checksums.txt.pem" \
  --signature   "loglens_${VERSION#v}_checksums.txt.sig" \
  --certificate-identity-regexp "https://github.com/malagant/loglens/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "loglens_${VERSION#v}_checksums.txt"

sha256sum -c "loglens_${VERSION#v}_checksums.txt" --ignore-missing
```

A successful verify proves the checksums file was signed by our exact release workflow and was logged in the public [Rekor](https://docs.sigstore.dev/logging/overview/) transparency log.

## Known limitations (v0.1)

LogLens v0.1 is honest about its scope. Filed as pinned issues on the tracker:

- **`kubectl` shell-out, not native `client-go`.** Inherits your existing kube auth, but adds a process per source. Native `client-go` is on the v0.2 roadmap.
- **No journald, Docker, or CloudWatch sources yet.** Only `file://` and `k8s://` ship in v0.1. The source interface is pluggable — see [`internal/source`](internal/source) for the contract.
- **Filter DSL is intentionally small.** No booleans, regex literals, time-window predicates, or aggregations until v0.2. The grammar table above is the complete language.
- **No persistent buffer or search-back.** The ring buffer is in-memory; restart loses scroll-back. v0.2 will add an optional on-disk session log.
- **No mouse support.** Keyboard-driven by design. Mouse selection works in your terminal (OSC 52 copy is supported).

If something on this list blocks you, comment on the corresponding pinned issue — usage signal will reorder the roadmap.

## Community & feedback

- **GitHub Discussions** — [github.com/malagant/loglens/discussions](https://github.com/malagant/loglens/discussions) for questions, ideas, and source/scheme requests.
- **GitHub Issues** — [github.com/malagant/loglens/issues](https://github.com/malagant/loglens/issues) for bugs and concrete feature requests.
- **Discord** — coming soon; an invite link will be posted here and pinned in Discussions once the server is up.

No telemetry, no phone-home, no analytics. The only feedback channel is you talking to us in the open.

## Contributing

We build in the open and welcome PRs. Start with [CONTRIBUTING.md](CONTRIBUTING.md) — it covers the build/test/lint loop, the release process, and the commit-message style. Good first issues are labeled [`good-first-issue`](https://github.com/malagant/loglens/issues?q=label%3Agood-first-issue).

## License

[MIT](LICENSE)
