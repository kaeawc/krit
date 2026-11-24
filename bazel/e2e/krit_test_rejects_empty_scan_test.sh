#!/usr/bin/env bash
# A krit_test whose sources krit skips must fail, not pass vacuously.
set -euo pipefail
if out=$("$1" 2>&1); then
  echo "$out"
  echo "krit_test passed although krit analyzed no files" >&2
  exit 1
fi
echo "$out"
case "$out" in
  *"krit_test: krit analyzed none of the sources"*) ;;
  *) echo "krit_test failed for another reason than the empty scan" >&2; exit 1 ;;
esac
