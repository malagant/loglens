# LogLens

One TUI, every log source, one query language.

LogLens is a fast, local-only terminal UI for tailing and searching logs across Kubernetes, local files, journald, and Docker — with a single filter DSL, JSON drill-down, and request-id correlation. No phone-home, no backend, just your terminal.

> Status: pre-alpha. The first tagged release is `v0.0.1-alpha` and is a scaffold only. See the roadmap in [SPA-6](https://example.invalid/SPA/issues/SPA-6).

## Install

```sh
# macOS / Linux (Homebrew tap, once published)
brew install loglens/tap/loglens

# Windows (Scoop bucket, once published)
scoop bucket add loglens https://github.com/loglens/scoop-bucket
scoop install loglens

# Anywhere with Go 1.23+
go install github.com/loglens/loglens/cmd/loglens@latest
```

## Demo

![asciinema demo placeholder](docs/demo.gif)

A real recording lands with the first feature-complete alpha. See `docs/` for the placeholder file.

## Why

`stern` is Kubernetes-only. CloudWatch only opens in a browser. Datadog costs $200/host. `lnav` is brilliant but file-only. Every DevOps engineer ends up juggling four log tools a day. LogLens is the unifier — one keymap, one filter language, every source you own.

## Contributing

We build in the open and welcome PRs. Start with [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues are labeled [`good-first-issue`](https://github.com/loglens/loglens/issues?q=label%3Agood-first-issue).

## License

[MIT](LICENSE)
