# LogLens docs

Artifacts shipped with the repo:

- [`demo.gif`](demo.gif) — short TUI walkthrough rendered from [`demo.tape`](demo.tape)
  via [VHS](https://github.com/charmbracelet/vhs). Embedded in the main README.
- [`demo.cast`](demo.cast) — asciinema v3 cast generated from [`cast.sh`](cast.sh)
  for plaintext / no-JS / screen-reader contexts.
- [`demo.tape`](demo.tape) / [`cast.sh`](cast.sh) — source-of-truth scripts. Re-render
  both with `make demo` (or `make demo-gif` / `make demo-cast` individually).
- [`sample.log`](sample.log) — small JSON log used by both demo scripts so the
  recording is reproducible without a live source.

## Future docs (post-v0.1.0)

- `architecture.md` — adapter interface, merge pipeline, filter DSL grammar.
- `installation.md` — Homebrew / Scoop / Linux package manager flows in depth.
