#!/bin/sh
# kubectl-stub.sh — minimal kubectl replacement for the k8s source unit tests.
#
# It speaks just enough of `kubectl get pods` and `kubectl logs` to drive the
# Source through its happy and sad paths without a live cluster. State is
# passed via env vars so each test can configure its scenario:
#
#   STUB_PODS_FILE    File whose lines are pod names returned by `get pods`.
#   STUB_LOGS_DIR     Directory of `<pod>` files containing log lines for
#                     `kubectl logs <pod>`. Missing file → empty output.
#   STUB_LOGS_FAIL    If non-empty, `kubectl logs` exits 1 with the value on
#                     stderr (simulates kubeconfig / API errors).
#   STUB_GET_FAIL     If non-empty, `kubectl get pods` exits 1 with the value
#                     on stderr.
#   STUB_LOGS_HANG    If non-empty, sleep forever after streaming the canned
#                     lines instead of exiting — simulates -f keeping the
#                     connection open.

set -e

# Find the subcommand. Skip global flags so we don't trip on `--context X` or
# `-n NS` placed before the subcommand.
cmd=""
while [ $# -gt 0 ]; do
  case "$1" in
    --context|--namespace|-n|--kubeconfig)
      shift 2 || exit 2
      continue
      ;;
    -*)
      shift
      continue
      ;;
    *)
      cmd="$1"
      shift
      break
      ;;
  esac
done

case "$cmd" in
  get)
    if [ -n "$STUB_GET_FAIL" ]; then
      printf '%s\n' "$STUB_GET_FAIL" >&2
      exit 1
    fi
    if [ -n "$STUB_PODS_FILE" ] && [ -f "$STUB_PODS_FILE" ]; then
      while IFS= read -r p; do
        [ -z "$p" ] && continue
        printf 'pod/%s\n' "$p"
      done < "$STUB_PODS_FILE"
    fi
    ;;
  logs)
    # The last positional argument is the pod name (after the subcommand and
    # its flags). Walk to the end.
    pod=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --context|--namespace|-n|--kubeconfig|-c|--container)
          shift 2 || exit 2
          continue
          ;;
        -*)
          shift
          continue
          ;;
        *)
          pod="$1"
          shift
          ;;
      esac
    done
    if [ -n "$STUB_LOGS_FAIL" ]; then
      printf '%s\n' "$STUB_LOGS_FAIL" >&2
      exit 1
    fi
    file="$STUB_LOGS_DIR/$pod"
    if [ -f "$file" ]; then
      cat "$file"
    fi
    if [ -n "$STUB_LOGS_HANG" ]; then
      # Sleep on a long timer; the parent's ctx-cancel will kill us.
      sleep 3600
    fi
    ;;
  *)
    printf 'kubectl-stub: unknown subcommand %s\n' "$cmd" >&2
    exit 2
    ;;
esac
