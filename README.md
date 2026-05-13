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

## Verifying release artifacts

Releases are signed with [Sigstore](https://www.sigstore.dev/) **keyless cosign**: the signing identity is the GitHub Actions workflow itself, not a long-lived keypair. To verify the checksums file for a release:

```sh
VERSION=v0.0.2-alpha
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

## Contributing

We build in the open and welcome PRs. Start with [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues are labeled [`good-first-issue`](https://github.com/loglens/loglens/issues?q=label%3Agood-first-issue).

## License

[MIT](LICENSE)
