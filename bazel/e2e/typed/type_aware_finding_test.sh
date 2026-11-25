#!/usr/bin/env bash
# A type-aware krit_test reports a finding only FIR can prove: the type comes
# from another target's jar on Bazel's compile classpath.
set -euo pipefail
if out=$("$1" 2>&1); then
  echo "$out"
  echo "type-aware krit_test passed although FIR should report UnnecessarySafeCall" >&2
  exit 1
fi
echo "$out"
case "$out" in
  *"typed/app/Consumer.kt:5:"*UnnecessarySafeCall*) ;;
  *) echo "type-aware krit_test failed without the FIR finding" >&2; exit 1 ;;
esac
