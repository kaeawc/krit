#!/usr/bin/env bash
# Checks a krit_project_report's plain report: report files, then --, then
# the substrings it must contain.
set -euo pipefail
plain=""
while [ "$#" -gt 0 ] && [ "$1" != -- ]; do
  case "$1" in
    *.krit.txt) plain="$1" ;;
  esac
  shift
done
shift
cat "$plain"
for want in "$@"; do
  grep -qF -- "$want" "$plain" || {
    echo "missing in the project report: $want" >&2
    exit 1
  }
done
