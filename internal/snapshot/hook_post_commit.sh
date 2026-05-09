#!/usr/bin/env bash
#
# Post-commit hook: capture a structural snapshot for the new HEAD.
#
# Runs `krit snapshot capture HEAD` detached so a slow or failing
# capture never blocks `git commit`. A failure leaves no snapshot for
# the new sha; rerun manually with `krit snapshot capture HEAD` to
# fill the gap.

set -u

KRIT_BIN=${KRIT_BIN:-krit}

if ! command -v "$KRIT_BIN" >/dev/null 2>&1; then
    exit 0
fi

LOG_DIR=".krit/snapshots"
LOG_FILE="$LOG_DIR/post-commit.log"
mkdir -p "$LOG_DIR" 2>/dev/null || true

nohup "$KRIT_BIN" snapshot capture HEAD >>"$LOG_FILE" 2>&1 </dev/null &
disown 2>/dev/null || true

exit 0
