#!/usr/bin/env bash
# idle-cpu.sh — sample the CPU% of a running loglens process over 10 seconds
# and print the average.
#
# Usage:
#   ./scripts/idle-cpu.sh [pid]
#
# If no PID is supplied the script looks for a running loglens process.
# On macOS it uses `top -l` (log mode); on Linux it uses `ps` in a loop.
#
# Exit codes:
#   0 — success, average printed to stdout
#   1 — loglens process not found or sample loop produced no data

set -euo pipefail

SAMPLE_SECONDS=10
SAMPLE_INTERVAL=1

pid="${1:-}"

if [[ -z "$pid" ]]; then
  pid="$(pgrep -x loglens 2>/dev/null | head -1 || true)"
  if [[ -z "$pid" ]]; then
    echo "idle-cpu: no loglens process found (start loglens first, or pass a PID)" >&2
    exit 1
  fi
fi

# Verify the PID exists before sampling.
if ! kill -0 "$pid" 2>/dev/null; then
  echo "idle-cpu: PID $pid does not exist" >&2
  exit 1
fi

echo "idle-cpu: sampling PID $pid for ${SAMPLE_SECONDS}s ..." >&2

samples=()

case "$(uname -s)" in
  Darwin)
    # top -l N: log N samples at 1-second intervals, suppress header noise.
    # Filter lines that start with our PID.
    while IFS= read -r line; do
      cpu="$(awk -v pid="$pid" '$1 == pid { print $3 }' <<< "$line")"
      [[ -n "$cpu" ]] && samples+=("${cpu%\%}")
    done < <(top -l "$SAMPLE_SECONDS" -s "$SAMPLE_INTERVAL" -stats pid,command,cpu -pid "$pid" 2>/dev/null | grep -E "^$pid ")
    ;;
  Linux)
    for (( i=0; i<SAMPLE_SECONDS; i++ )); do
      cpu="$(ps -p "$pid" -o %cpu= 2>/dev/null | tr -d ' ' || true)"
      if [[ -z "$cpu" ]]; then
        echo "idle-cpu: PID $pid exited during sampling" >&2
        break
      fi
      samples+=("$cpu")
      sleep "$SAMPLE_INTERVAL"
    done
    ;;
  *)
    echo "idle-cpu: unsupported OS $(uname -s)" >&2
    exit 1
    ;;
esac

if [[ ${#samples[@]} -eq 0 ]]; then
  echo "idle-cpu: no samples collected — is loglens still running?" >&2
  exit 1
fi

# Compute average with awk.
avg="$(printf '%s\n' "${samples[@]}" | awk '{ sum += $1; n++ } END { printf "%.2f", sum/n }')"
echo "idle-cpu: average CPU% over ${#samples[@]} samples: $avg%" >&2
echo "$avg"
