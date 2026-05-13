# LogLens v0.1 — one TUI, every log source, one query language

*Status: draft for CEO review. Do not publish until approved.*

![LogLens demo — tail a file, filter for errors, drill into a request](demo.gif)

`brew install malagant/tap/loglens` — that is the install line. The rest of this post is why we wrote it.

## The pain

Every DevOps engineer I have ever worked with juggles at least four log tools in a workday.

`kubectl logs -f` for the pod you are debugging right now. `stern` for the deployment, because `kubectl` only does one pod at a time and the truncated label-selector trick never quite works the way the docs say. CloudWatch in a browser tab, because that is where the API gateway lives and there is no good way around it. `lnav` for the local file from the customer's bug report. `tail -F` for the third-party container that does not log to stdout because of course it does not. Datadog in *another* browser tab if your employer paid the bill. Loki in a third if they did not.

Each tool has its own keymap. Its own filter language. Its own definition of "error". `stern` uses regexes. `kubectl` uses a flag soup that approximates `grep`. CloudWatch Logs Insights is a SQL-shaped DSL that is almost — but not quite — a real query language. Datadog has facets. Loki has LogQL. And every one of them speaks UTC timestamps in a slightly different format.

The Unix philosophy got us the first half of the way: small composable tools, one per source. The second half — *making them feel like a single workspace* — never happened. You compose by alt-tabbing between four terminal windows and three browser tabs.

I have watched engineers go through this loop on a Friday at 22:00 with a P1 incident hot and a CEO Slacking them every six minutes. It is not a productive place to be.

## The wedge

LogLens is a fast, local TUI that tails *every log source you own* — Kubernetes pods, local files, journald, Docker (the last two on the v0.2 list) — into a single merged scroll-back with one keymap and one filter language.

That is the entire pitch. One window. One keymap. One filter. Every source you have.

The first cut, shipping today as `v0.1.0`, supports two source schemes:

- `file://` — tails a single file with full `tail -F` semantics: survives truncation, survives inode-swap rotation, picks up new content when the writer reopens.
- `k8s://[<context>/]<namespace>/<pod-or-selector>` — tails one pod or every pod matching a label selector by shelling out to `kubectl logs -f --timestamps --all-containers --prefix`. New pods that match a selector mid-stream are picked up automatically.

You can pass `--source` as many times as you want. They merge, in timestamp order, into one stream:

```sh
loglens \
  --source file:///var/log/app.log \
  --source k8s://default/api-7d8c9b6f4-x2k9z \
  --source k8s://prod-eu/app=checkout
```

Press `/` to open the filter prompt. Type a predicate. The stream narrows live as you type:

```text
level=error                       # only errors
level=warn source=k8s             # warnings from any kubernetes source
/request id=abc/                  # substring with spaces
field.user_id=42 !source=staging  # field match, excluding staging
```

The filter language is intentionally small — five predicate kinds, all ANDed together, with `!` to negate. That is the entire grammar. No booleans, no regex literals, no time windows. We picked the smallest grammar that covers ~90% of real incident-shell queries and shipped it. The remaining 10% is on the v0.2 list, and we will not ship a bigger DSL until we have seen which 10% you actually need.

Press Enter on a row to drop the JSON detail into the right pane: a collapsible tree, `j`/`k` to walk, `l`/`h` to expand and collapse, `y` to copy the value via OSC 52 (works over SSH, works in tmux). LogLens auto-detects a request id in the highlighted row and inline-accents every other row that shares it — the correlator field is configurable with `--correlate`. Drilling down from a 500 to "every line that this request touched across three services" takes about two seconds.

## Why a TUI

Because the alternative to alt-tabbing between four tools is not a fifth web app. It is a single window, in the same terminal you already have open, that does the merge for you.

Web dashboards lose on three counts:

- They are not in your pane. Your terminal is.
- They run in someone else's process and someone else's cost center. LogLens is one binary, no backend, no daemon.
- They cannot answer "what was the last 200 lines of this pod, intersected with the last error from this file, while I am inside `kubectl exec` in a third pane." A TUI can.

Modern terminal UI libraries — Bubble Tea, Lipgloss, Bubbles from charm.sh in our case — have closed the gap. Color, animation, layout, mouse, OSC 52 clipboard, kitty graphics if you want them. We measured every part of the v0.1 build against a hard performance budget:

| Metric | Budget | Measured (macOS arm64) |
| --- | --- | --- |
| Cold start (exec → TUI ready) | < 100 ms | ~16 ms |
| Key-response (filter match, worst case) | < 16 ms | < 0.01 ms (~130 M predicate evaluations/s) |
| Idle CPU | ≈ 0% | 0.0% |

All three have a regression guard in CI. The matcher guard fails the build if throughput on a representative compound predicate drops more than 30% below the recorded per-platform baseline. We hold ourselves to this every commit.

## Why local, why no telemetry

LogLens does not phone home. Not for crash reports, not for usage stats, not for "anonymous" feature signal. There is no analytics SDK and there will never be one.

This is partly principle — your logs are yours, full stop. It is also pragmatic. The single most common deployment context for this tool is "incident, 22:00, in a customer's prod cluster on an air-gapped VPN with a senior engineer on shared screen behind you." Any tool that opens an unexpected outbound socket in that context gets uninstalled, and rightly so.

Feedback comes from GitHub Issues, GitHub Discussions, and humans talking to humans. That is the whole loop.

## The install line

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
```

Pre-built archives for linux/macos/windows × amd64/arm64 are attached to every GitHub Release, together with a SHA-256 checksums file and a **keyless cosign signature** tied to the release workflow's GitHub Actions identity. You can verify the signature against the public Rekor transparency log without us ever holding a private key. The exact `cosign verify-blob` recipe is in the [README](https://github.com/malagant/loglens#verifying-release-artifacts).

## What is on purpose missing

Being honest about scope is part of the brand:

- **No journald, Docker, or CloudWatch sources yet.** The `source://` registry is pluggable — see [`internal/source`](https://github.com/malagant/loglens/tree/main/internal/source). The next three sources are on the v0.2 list in that order.
- **`kubectl` shell-out, not native `client-go`.** Trade-off: we inherit whatever kube auth you already have working (oidc, aws-iam-authenticator, gke-gcloud-auth-plugin) without bundling them. Cost: one process per pod source. Native client-go is on the v0.2 roadmap.
- **No persistence.** The ring buffer is in-memory; restart loses scroll-back. An optional on-disk session log is on the v0.2 list.
- **No mouse support.** Keyboard by design. Terminal selection and OSC 52 copy still work.

If any of those blocks you, the corresponding pinned issue on the tracker is the right place to say so. Usage signal will reorder the roadmap.

## The roadmap, briefly

v0.2 is the next public milestone:

- Native `client-go` Kubernetes source.
- `journald://` and `docker://` schemes.
- Boolean/regex/time-window operators in the filter DSL.
- Persistent session log.
- Configurable column layout.

We will ship it when it is ready. There is no release cadence, no marketing pressure, no SLA. There is a backlog, a small team, and a high bar.

## Thanks

To the Charm.sh team for Bubble Tea and Lipgloss; to the GoReleaser, Sigstore, and Rekor projects for making keyless signed releases boring; to the early reviewers who told us in plain language which two features to cut from v0.1.

If LogLens replaces even one of your browser tabs, we have done our job. If it replaces three, please star the repo — that is the only metric we look at.

— The LogLens team
