#!/usr/bin/env bash
set -euo pipefail

# Runs krit analysis inside the GitHub Action.
# Expected environment variables:
#   KRIT_CONFIG         - path to config file (optional, may be empty)
#   KRIT_CACHE_ANALYSIS - "true" to enable incremental cache
#   KRIT_REPORTER       - reporter mode: sarif, github-annotations, none
#   KRIT_SARIF_FILE     - path for SARIF output
#   KRIT_EXTRA_ARGS     - additional CLI arguments
#   KRIT_PATHS          - space-separated paths to scan
#   KRIT_SARIF_UPLOAD   - "true" to generate SARIF for upload
#   KRIT_FAIL_ON_FINDINGS - "true" to exit 1 when findings exist

set +e

ARGS=""

# Config
if [ -n "${KRIT_CONFIG:-}" ]; then
  ARGS="$ARGS --config $KRIT_CONFIG"
fi

# Incremental cache
if [ "${KRIT_CACHE_ANALYSIS:-}" = "true" ]; then
  ARGS="$ARGS --cache-dir .krit-cache"
fi

# Diff mode (only check changed lines)
if [ -n "${KRIT_DIFF:-}" ]; then
  ARGS="$ARGS --diff $KRIT_DIFF"
fi

# Determine output format based on reporter
REPORTER="${KRIT_REPORTER:-github-annotations}"
SARIF_FILE="${KRIT_SARIF_FILE:?}"

if [ "$REPORTER" = "sarif" ]; then
  # shellcheck disable=SC2086
  krit $ARGS --report=sarif -o "$SARIF_FILE" ${KRIT_EXTRA_ARGS:-} ${KRIT_PATHS:-.}
  EXIT_CODE=$?

  # Count findings from SARIF
  if [ -f "$SARIF_FILE" ]; then
    COUNT=$(python3 -c "
import json, sys
with open('$SARIF_FILE') as f:
    data = json.load(f)
print(sum(len(run.get('results', [])) for run in data.get('runs', [])))
" 2>/dev/null || echo "0")
  else
    COUNT=0
  fi

  {
    echo "findings-count=$COUNT"
    echo "sarif-file=$SARIF_FILE"
    echo "exit-code=$EXIT_CODE"
  } >> "$GITHUB_OUTPUT"

  if [ "${KRIT_FAIL_ON_FINDINGS:-}" = "true" ] && [ "$COUNT" -gt 0 ]; then
    exit 1
  fi

elif [ "$REPORTER" = "github-annotations" ]; then
  # Run with plain output for problem matcher annotations
  # shellcheck disable=SC2086
  PLAIN_OUTPUT=$(krit $ARGS --report=plain ${KRIT_EXTRA_ARGS:-} ${KRIT_PATHS:-.} 2>&1)
  PLAIN_EXIT=$?

  # Print output so problem matcher can pick up annotations
  echo "$PLAIN_OUTPUT"

  # Generate SARIF for Code Scanning upload
  if [ "${KRIT_SARIF_UPLOAD:-}" = "true" ]; then
    # shellcheck disable=SC2086
    krit $ARGS --report=sarif -o "$SARIF_FILE" ${KRIT_EXTRA_ARGS:-} ${KRIT_PATHS:-.} > /dev/null 2>&1
  fi

  # Count findings
  COUNT=$(echo "$PLAIN_OUTPUT" | grep -c "^.\+:[0-9]\+:[0-9]\+:" || true)
  {
    echo "findings-count=$COUNT"
    echo "sarif-file=$SARIF_FILE"
    echo "exit-code=$PLAIN_EXIT"
  } >> "$GITHUB_OUTPUT"

  if [ "${KRIT_FAIL_ON_FINDINGS:-}" = "true" ] && [ "$COUNT" -gt 0 ]; then
    exit 1
  fi

else
  # reporter=none: just run and capture exit code
  # shellcheck disable=SC2086
  krit $ARGS --report=plain ${KRIT_EXTRA_ARGS:-} ${KRIT_PATHS:-.} > /dev/null 2>&1
  EXIT_CODE=$?
  echo "findings-count=0" >> "$GITHUB_OUTPUT"
  echo "exit-code=$EXIT_CODE" >> "$GITHUB_OUTPUT"
  if [ "${KRIT_FAIL_ON_FINDINGS:-}" = "true" ] && [ "$EXIT_CODE" -ne 0 ]; then
    exit 1
  fi
fi
