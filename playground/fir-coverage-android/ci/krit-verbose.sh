#!/usr/bin/env bash
# Stand-in krit binary for kritCheck (-PkritBinary=.krit-bin/krit-verbose).
# kritCheck runs krit with -q; this runs the krit next to it with -v instead
# and keeps its stderr, which carries the "FIR verdict" coverage line, in
# kritcheck-<pid>.log beside it. The plugin still gets krit's exit code and
# report.
set -uo pipefail
dir="$(cd "$(dirname "$0")" && pwd)"
args=()
for arg in "$@"; do
  if [ "$arg" = "-q" ]; then arg="-v"; fi
  args+=("$arg")
done
"$dir/krit" "${args[@]}" 2> >(tee "$dir/kritcheck-$$.log" >&2)
rc=$?
wait
exit "$rc"
