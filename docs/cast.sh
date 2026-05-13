#!/usr/bin/env bash
# Driver script for the asciinema fallback (docs/demo.cast).
#
# The TUI walkthrough lives in docs/demo.gif (rendered via `vhs docs/demo.tape`).
# This cast is the text/no-JS fallback for readers who can't render GIFs:
# man pages, plaintext previews, screen readers via `asciinema cat`, etc.
#
# Re-record from the repo root with:
#
#   asciinema rec --rows 28 --cols 110 --overwrite \
#     -c "docs/cast.sh" docs/demo.cast
#
# Requires `loglens` on PATH (`make build && export PATH=$PWD/dist:$PATH`).

set -e
PS_DELAY="${PS_DELAY:-0.5}"

show() {
  printf '\033[36m$\033[0m %s\n' "$*"
  sleep "$PS_DELAY"
  eval "$@"
  echo
  sleep "$PS_DELAY"
}

show 'loglens --version'
show 'head -3 docs/sample.log'

# LOGLENS_DUMP=1 streams the merged pipeline to stdout (no TUI), which is what
# `asciinema cat docs/demo.cast` can actually render. The interactive TUI flow
# (filter prompt, detail pane, help overlay) is captured separately in
# docs/demo.gif.
printf '\033[36m$\033[0m %s\n' 'LOGLENS_DUMP=1 loglens --source file://docs/sample.log'
sleep "$PS_DELAY"
LOGLENS_DUMP=1 loglens --source file://docs/sample.log &
PID=$!
sleep 1.5
kill -INT $PID 2>/dev/null || true
wait $PID 2>/dev/null || true

sleep "$PS_DELAY"
printf '\033[2m# For the interactive TUI (filter language, detail pane), see docs/demo.gif.\033[0m\n'
sleep 1
