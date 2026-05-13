# Launch posts — HN / Lobsters / r/devops

*Status: drafts queued for CEO posting on launch day. Do not publish until approved.*

All three reference the same launch post (`docs/LAUNCH.md`) and the same install line. The differences are tone and length, tuned per audience.

---

## Hacker News

**Title (suggested, ≤ 80 chars):**

```
LogLens – one TUI, every log source, one query language
```

**URL (post as link, not text):** `https://github.com/malagant/loglens`

**First comment (post immediately after submission; HN treats the first author comment as the de-facto intro):**

```
Hi HN — I'm the author. LogLens is a fast local TUI that tails Kubernetes
pods, local files, journald, and Docker (last two on the v0.2 list) into
one merged scroll-back with one keymap and one filter language.

Why: every devops engineer I work with juggles `kubectl logs`, `stern`,
`lnav`, `tail -F`, and at least one browser tab a day. Each has its own
keymap and its own filter dialect. LogLens is the unifier.

v0.1.0 ships today:

  - file:// and k8s:// sources (kubectl shell-out, inherits your existing
    kube auth — native client-go is v0.2)
  - five-predicate filter DSL with live-as-you-type preview
  - JSON drill-down with request-id correlation
  - cold start ~16ms, idle CPU 0%, matcher ~130M ops/s (all CI-guarded)
  - keyless cosign-signed release artifacts, Homebrew / Scoop / GHCR
  - no telemetry, no backend, MIT licensed

Install:

  brew install malagant/tap/loglens

Honest about what's missing: no journald/Docker/CloudWatch sources yet,
no booleans/regex in the filter, no persistent buffer. All pinned issues
on the tracker. If a missing source blocks you, that's the right place
to comment — usage signal reorders the roadmap.

Built with Go + Charm.sh's Bubble Tea/Lipgloss. Single binary, ~7MB.

Demo GIF: https://github.com/malagant/loglens#demo
Launch post: https://github.com/malagant/loglens/blob/main/docs/LAUNCH.md

Happy to answer anything — esp. about the source plugin model, the
filter grammar trade-offs, or why we picked Bubble Tea over Ratatui.
```

**Posting notes for CEO:**

- Post **Tuesday–Thursday, 09:00–10:30 UTC** for best US/EU overlap.
- Submit the GitHub URL, not the launch-post URL — HN penalises blog-shaped submissions for new accounts.
- After submission, paste the first comment within 60 seconds.
- Stay on the thread for the first 3 hours — answering one question well in the first hour matters more than the next twelve combined.
- Do not vote-ring, do not ask anyone to upvote. HN moderators will downrank instantly and irreversibly.
- If the title gets edited by mods, do not re-submit; comment thanking them and move on.

---

## Lobste.rs

**Title:**

```
LogLens: one TUI, every log source, one query language
```

**Tags (suggested):** `devops`, `release`, `go`, `unix`

**URL:** `https://github.com/malagant/loglens`

**Description (the field Lobsters renders inline below the link):**

```
A fast, local terminal UI that tails Kubernetes pods, local files,
journald, and Docker into one merged scroll-back with a single
keymap and a single filter language. v0.1.0 ships file:// and k8s://
sources (kubectl shell-out, native client-go is on the v0.2 list);
journald, Docker, and CloudWatch are next.

Built in Go with Charm.sh's Bubble Tea/Lipgloss. Single static
binary, ~7MB, MIT, no telemetry, no backend. Releases are
keylessly signed with Sigstore cosign and logged in Rekor; the
verify recipe is in the README.

Why it exists: every devops engineer I work with juggles `stern`,
`kubectl logs`, `lnav`, CloudWatch, and a couple of browser tabs
a day, each with its own keymap and filter dialect. LogLens is
the unifier — one window, one keymap, one query language, every
source you own.

Honest about scope: no journald/Docker source yet, no
booleans/regex in the filter, no persistent buffer. Tracked as
pinned issues; the v0.1 README is explicit about what's missing.

Launch write-up:
https://github.com/malagant/loglens/blob/main/docs/LAUNCH.md
```

**Posting notes:**

- Lobsters rewards focused technical writeups. Keep the description ≤ 200 words. Above is ~180.
- The `release` tag is the right primary; `go` and `unix` are supporting. Do not over-tag.
- Lobsters culture: be a participant before/after, not just a poster. If the CEO has not commented elsewhere recently, expect a slow start.
- Disagreement is more pointed and more knowledgeable than HN. Engage on the merits, not the framing.

---

## r/devops

**Title:**

```
[OC] I built LogLens — one TUI that tails kubectl, files, journald, and Docker into one window with one keymap. v0.1 just shipped.
```

**Body:**

```markdown
Hey r/devops — I built this because I got tired of having `stern`,
`kubectl logs`, `lnav`, CloudWatch, and a Datadog tab all open at
once during incidents. Each one has its own keymap, its own filter
dialect, and its own definition of "error". LogLens is a fast TUI
that merges them into one scroll-back.

**v0.1.0 (today):**

- `file://` source — `tail -F` semantics, survives truncation and rotation
- `k8s://` source — single pod or label-selector, new pods picked up mid-stream, shells out to `kubectl logs` so you inherit your existing auth (oidc, aws-iam-authenticator, gke-gcloud-auth-plugin)
- One filter language across all sources: `level=error source=k8s /upstream/ field.user_id=42 !source=staging`
- JSON drill-down + request-id correlation (drill from a 500 to every line that request touched)
- Cold start ~16ms, matcher ~130M predicate evaluations/sec, idle CPU 0% — all CI-guarded
- Keyless cosign-signed releases (Sigstore + GitHub Actions OIDC, verifiable against Rekor)
- Single static binary, MIT, no telemetry, no backend

**Install:**

    brew install malagant/tap/loglens
    # or
    scoop bucket add malagant https://github.com/malagant/scoop-bucket && scoop install loglens
    # or
    docker run --rm -it ghcr.io/malagant/loglens:latest --help

**Demo GIF + the why:**
https://github.com/malagant/loglens

**What's still missing (honest list):**

- No native `client-go` source yet (kubectl shell-out works but adds a process per pod). v0.2.
- No journald, Docker, or CloudWatch source schemes yet. v0.2.
- No booleans/regex/time-windows in the filter DSL. v0.2.
- No persistent buffer; restart loses scroll-back. v0.2.

If any of those blocks you, comment on the pinned issue on the tracker — that's how the roadmap gets reordered. Happy to answer anything in the thread.
```

**Posting notes:**

- r/devops mods downrank or remove anything that reads as marketing. Lead with the problem, not the product. The `[OC]` tag is required for self-promotion.
- Post **Tuesday–Wednesday, 13:00–15:00 UTC** (catches both EU and US-East engineers on lunch).
- Reply to every top-level comment within the first 4 hours. r/devops karma rewards engagement weight.
- Cross-post to r/kubernetes and r/sre **only if** the r/devops post is not removed within 30 min. Re-posting too fast trips Reddit's site-wide spam filter.

---

## Pre-launch checklist (CEO)

- [ ] Approve `docs/LAUNCH.md`.
- [ ] Confirm the three drafts above (titles + bodies). Tone tweaks are fine; structure has been tuned per platform.
- [ ] Confirm the launch day (target: Tuesday or Wednesday in the launch week).
- [ ] CTO confirms tap/bucket/GHCR installs are smoke-tested on a clean macOS + clean Windows + clean Linux box on the morning of launch.
- [ ] CTO is on-call (issue triage, comment monitoring) for the first 72h after the HN submission goes up.

## Day-of sequence (CEO posts; CTO monitors)

1. T-0:00 — Submit HN.
2. T-0:01 — Drop the first comment on HN.
3. T-0:05 — Submit Lobsters.
4. T-0:15 — Post r/devops.
5. T+0:30 — Re-share to personal LinkedIn / Twitter / Mastodon (links only, not the launch post text).
6. T+3:00 — First check-in: CTO summarizes the top-of-thread feedback themes to CEO in a Paperclip comment.
7. T+24:00, T+48:00, T+72:00 — Triage rollups posted to the launch issue thread.
