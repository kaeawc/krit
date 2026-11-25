#!/usr/bin/env bash
# Run krit for the root action.yml and report a SARIF file and its result
# count. Needs only bash and jq (preinstalled on GitHub-hosted runners).
#
# env:
#   KRIT_PATHS        Space-separated paths to scan (default ".").
#   KRIT_ARGS         Extra CLI arguments, word-split.
#   KRIT_CONFIG       --config path (optional).
#   KRIT_BASELINE     --baseline path (optional).
#   KRIT_SARIF        SARIF output path.
#   KRIT_BASE_PATH    --base-path for relative SARIF locations (default: the
#                     workspace).
#   KRIT_FAIL_ON_FINDINGS  "true" to fail the step when SARIF has results.

set -uo pipefail

: "${KRIT_SARIF:?KRIT_SARIF is required}"
# --no-daemon: a one-shot CI scan gains nothing from a resident daemon, which
# would outlive the step. On macOS the daemon's kqueue watcher opens a
# descriptor per watched file and, on a large checkout, exhausts the process
# limit; the daemon then reports zero findings instead of failing.
args=(--no-daemon --report=sarif -o "$KRIT_SARIF" --base-path "${KRIT_BASE_PATH:-${GITHUB_WORKSPACE:-$PWD}}")
if [ -n "${KRIT_CONFIG:-}" ]; then
  args+=(--config "$KRIT_CONFIG")
fi
if [ -n "${KRIT_BASELINE:-}" ]; then
  args+=(--baseline "$KRIT_BASELINE")
fi
# shellcheck disable=SC2206 # KRIT_ARGS and KRIT_PATHS are word lists by design.
extra=(${KRIT_ARGS:-})
# shellcheck disable=SC2206
paths=(${KRIT_PATHS:-.})

mkdir -p "$(dirname "$KRIT_SARIF")"
krit "${args[@]}" "${extra[@]}" "${paths[@]}"
code=$?

count=0
if [ -s "$KRIT_SARIF" ]; then
  if ! count=$(jq '[.runs[]?.results[]?] | length' "$KRIT_SARIF"); then
    echo "::error::krit wrote ${KRIT_SARIF}, but it is not valid SARIF JSON." >&2
    exit 1
  fi
elif [ "$code" -lt 2 ]; then
  echo "::error::krit exited ${code} without writing ${KRIT_SARIF}." >&2
  exit 1
fi

{
  echo "findings-count=${count}"
  echo "sarif-file=${KRIT_SARIF}"
  echo "exit-code=${code}"
} >> "${GITHUB_OUTPUT:-/dev/stdout}"
echo "krit: ${count} findings (exit ${code}); SARIF at ${KRIT_SARIF}"

# 2 and above are krit errors (bad flags, a failed --fail-on-degraded scan).
if [ "$code" -ge 2 ]; then
  exit "$code"
fi
if [ "${KRIT_FAIL_ON_FINDINGS:-true}" = "true" ] && [ "$count" -gt 0 ]; then
  exit 1
fi
exit 0
