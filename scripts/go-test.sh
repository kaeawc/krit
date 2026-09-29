#!/usr/bin/env bash
# Run `go test` under a process cap so a runaway spawn (a test binary
# re-launching itself, leaked daemons or JVMs) fails with fork errors
# instead of filling the machine's process table.
#
# `ulimit -u` counts every process the user owns, not just this run's,
# so the cap is the user's current process count plus a budget. Both the
# soft and hard limits are lowered, so nothing under this run can raise
# them back.
#
#   KRIT_TEST_PROC_BUDGET  extra processes allowed for this run (default 2048)
#   KRIT_TEST_PARALLEL     packages tested in parallel, go test -p (default 4)
#
# Arguments are passed to `go test` verbatim; a later -p overrides the default.
set -euo pipefail

budget="${KRIT_TEST_PROC_BUDGET:-2048}"
parallel="${KRIT_TEST_PARALLEL:-4}"

current=$(ps -u "$(id -u)" -o pid= | wc -l | tr -d ' ')
limit=$((current + budget))
hard=$(ulimit -H -u)
if [ "$hard" != "unlimited" ] && [ "$limit" -gt "$hard" ]; then
	limit=$hard
fi
ulimit -u "$limit"

exec go test -p "$parallel" "$@"
