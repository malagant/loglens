# Post-launch on-call playbook (72h)

The CTO is the on-call for the **first 72 hours** after the launch posts go up. The goal is fast, visible response — not heroics. A single "we're watching" comment in the first hour beats a perfect fix delivered three days later.

## Coverage

- **Hours 0–24:** CTO acknowledges every new issue / discussion / HN comment within **2 hours** (during waking hours; overnight gap is acceptable but post a banner before signing off).
- **Hours 24–72:** Acknowledgement SLA relaxes to **6 hours**. Critical bugs (install fails, segfault, data loss) still get a same-day response.
- After hour 72: revert to the normal cadence — issues triaged within a business day.

## Channels to watch

| Channel | What to watch for | Action |
| --- | --- | --- |
| `malagant/loglens` Issues | New bug reports, install failures, crash reports | Label, ack within SLA, fix in priority order |
| `malagant/loglens` Discussions | Questions, source-scheme requests, design feedback | Answer or convert to issue if it's a feature ask |
| `malagant/homebrew-tap` / `malagant/scoop-bucket` Issues | Install pipeline complaints | Mirror to main repo, fix the Formula/manifest |
| HN thread | Top-of-thread questions, criticisms | CEO replies on framing; CTO replies on technical questions |
| Lobsters thread | Same as HN but more technical | CTO replies in-thread |
| r/devops thread | Mostly use-case questions | CTO replies; convert feature asks to issues |

## Triage tags (apply on intake)

- `kind/bug` — something is broken vs. its documented behaviour.
- `kind/feature` — new functionality or new source scheme.
- `kind/docs` — README/CHANGELOG/CONTRIBUTING.
- `severity/crit` — install fails, panic on start, data corruption, signing chain broken.
- `severity/high` — feature broken in a common path.
- `severity/med` — broken in an edge case or workaround exists.
- `severity/low` — papercut, polish.
- `area/source-file`, `area/source-k8s`, `area/filter`, `area/jsonview`, `area/release` — what subsystem.

`crit` issues get a same-day patch release. `high` gets the next weekly. `med`/`low` go on the v0.2 board.

## Crit response template

```
Confirmed — this affects <scope>. Triaging now.

Workaround: <one line, if any>.

Tracking fix in <PR / issue link>. Will tag a patch release within 24h.
```

Post it within the SLA window even if the fix is not yet started. Visibility is the goal.

## Patch release loop

1. Open a PR titled `fix: <subsystem> <one-line summary>` against `main`.
2. Reference the issue in the PR body.
3. Verify the regression with a test (matcher bench, smoke, or source-specific unit) where reasonable.
4. Merge after CI green.
5. Tag `vX.Y.Z+1` from `main` — release workflow handles the rest.
6. Comment on the original issue with the release link and close it.

## Comment voice

- Plain language. Reference the failing command and the expected output.
- Acknowledge mistakes flat-out: "yeah, that's a bug — sorry, on it."
- Never blame the user's environment until you've reproduced the absence of the bug on a clean equivalent. Most "user error" reports are real bugs in disguise during a launch week.
- No emojis. No "Thanks for the feedback!" boilerplate. The user can tell.

## Rollups

CTO posts a rollup on the launch issue (`SPA-7`) at T+24h, T+48h, T+72h:

- New issues opened (count, by severity)
- New issues closed
- Patch releases cut (link)
- Top three themes from the threads (one line each)
- Anything trending toward "we should ship this in v0.1.1"

## Hand-off

After hour 72, post a final rollup on `SPA-7` and move the on-call line to "normal cadence" — `SPA-7` then closes as `done`. Open follow-up issues for everything the launch week surfaced; they belong on the v0.2 board, not in the launch ticket.
