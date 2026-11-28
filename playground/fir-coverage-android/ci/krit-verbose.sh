#!/usr/bin/env bash
# Stand-in krit binary for kritCheck (-PkritBinary=.krit-bin/krit-verbose).
# kritCheck runs krit with -q; this runs dist/krit next to it with -v instead
# and keeps its stderr, which carries the "FIR verdict" coverage line, in
# kritcheck-<pid>.log beside it, then copies it to stderr. The plugin still
# gets krit's exit code and report.
#
# stderr goes to the file, not through `2> >(tee ...)`: krit hands the
# pooled krit-fir daemon it starts every inherited descriptor, so the daemon
# kept tee's pipe open for its whole idle lifetime, tee never saw EOF, and
# the kritCheck task waiting on this script's stderr hung (#1752).
set -uo pipefail
dir="$(cd "$(dirname "$0")" && pwd)"
args=()
for arg in "$@"; do
  if [ "$arg" = "-q" ]; then arg="-v"; fi
  args+=("$arg")
done
log="$dir/kritcheck-$$.log"
"$dir/dist/krit" "${args[@]}" 2> "$log"
rc=$?
cat "$log" >&2
exit "$rc"
