# Contributing to LogLens

Thanks for helping build LogLens. This document covers the loop: build, test, release.

## Prerequisites

- Go 1.23 or newer (`go version`)
- `make` (optional, convenience targets)
- `golangci-lint` v1.61+ for linting (`brew install golangci-lint` or see [installation](https://golangci-lint.run/usage/install/))
- `goreleaser` v2+ for release dry-runs (`brew install goreleaser`)
- `cosign` for artifact signing in releases (`brew install cosign`)

## Build

```sh
go build ./...
```

The binary entry point is `cmd/loglens`. To run the hello-world TUI:

```sh
go run ./cmd/loglens
```

Press `q` or `Ctrl+C` to exit.

## Test

```sh
go test ./...
```

This includes a PTY smoke test under `internal/smoke` that boots the TUI inside a pseudo-terminal and verifies it renders without crashing.

## Lint

```sh
golangci-lint run
```

The config lives in `.golangci.yml`. Lint must be clean before merging.

## Release dry-run

Releases are tag-driven via GoReleaser. To verify your release config locally without publishing:

```sh
goreleaser release --snapshot --clean --skip=publish,sign
```

Artifacts land in `dist/`. To smoke a tagged release end-to-end (no signing keys required):

```sh
goreleaser release --snapshot --clean
```

A real tagged release runs in GitHub Actions on a `v*` push. Signing keys live in repo secrets (`COSIGN_PRIVATE_KEY`, `COSIGN_PASSWORD`).

## Commits and PRs

- One logical change per PR. Smaller is better.
- Conventional-commit style is preferred but not enforced (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`).
- Include a test for behaviour changes. UI changes get a VHS or asciinema recording when feasible.
- Sign commits if your environment supports it (`git commit -S`).

## Reporting bugs and proposing features

Use the issue templates:

- **Bug** — for reproducible defects
- **Feature** — for capability requests
- **Question** — for "how do I…" and design discussion

## Code of Conduct

Be kind. Assume good faith. We expect contributors to follow the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).
