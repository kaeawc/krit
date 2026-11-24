#!/usr/bin/env bash
# A krit_test over a file with a finding must fail and report it.
set -euo pipefail
if out=$("$1" 2>&1); then
  echo "$out"
  echo "krit_test passed on a file with a finding" >&2
  exit 1
fi
echo "$out"
case "$out" in
  *"src/Unsafe.kt:4:"*UnsafeCallOnNullableType*) ;;
  *) echo "krit_test failed without reporting the expected finding" >&2; exit 1 ;;
esac
