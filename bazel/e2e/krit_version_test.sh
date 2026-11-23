#!/usr/bin/env bash
set -euo pipefail
out=$("$1" --version)
echo "$out"
case "$out" in
  *[0-9].[0-9]*) ;;
  *) echo "unexpected krit --version output: $out" >&2; exit 1 ;;
esac
