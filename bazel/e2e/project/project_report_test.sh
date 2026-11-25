#!/usr/bin/env bash
# The project report has the dead code only a whole-build view can prove,
# and not the cross-target use a per-target view would get wrong.
set -euo pipefail
plain=""
for f in "$@"; do
  case "$f" in
    *.krit.txt) plain="$f" ;;
  esac
done
cat "$plain"
grep -q "project/lib/Api.kt:.*DeadCode.*'unusedApi'" "$plain" || {
  echo "the project report misses unusedApi's DeadCode finding" >&2
  exit 1
}
if grep -q "'usedApi'" "$plain"; then
  echo "the project report calls usedApi dead although //project/app calls it" >&2
  exit 1
fi
if grep -q "\[style:" "$plain"; then
  echo "the project report ran file-scope rules" >&2
  exit 1
fi
