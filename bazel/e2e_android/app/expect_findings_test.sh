#!/usr/bin/env bash
# Runs a krit_test that must fail, and checks its report has every expected
# substring.
set -euo pipefail
test="$1"
shift
if out=$("$test" 2>&1); then
  echo "$out"
  echo "krit_test passed although it should report findings" >&2
  exit 1
fi
echo "$out"
for want in "$@"; do
  case "$out" in
    *"$want"*) ;;
    *) echo "missing in the krit_test report: $want" >&2; exit 1 ;;
  esac
done
