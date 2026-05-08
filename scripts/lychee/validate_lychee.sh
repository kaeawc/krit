#!/usr/bin/env bash
# Run lychee link checker on documentation and markdown files.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

if ! command -v lychee >/dev/null 2>&1; then
  echo "lychee is not installed. Run scripts/lychee/install_lychee.sh first."
  exit 1
fi

echo "Running lychee link checker..."

lychee \
  --config "$ROOT/.lycherc.toml" \
  "$ROOT/docs/" \
  "$ROOT/README.md" \
  "$ROOT/CONTRIBUTING.md" \
  "$ROOT/CHANGELOG.md" \
  2>/dev/null || true

# Run with explicit exit code handling
lychee \
  --config "$ROOT/.lycherc.toml" \
  --format detailed \
  "$ROOT/docs/" \
  "$ROOT/README.md" \
  "$ROOT/CONTRIBUTING.md"

EXIT_CODE=$?

if [[ "$EXIT_CODE" -eq 0 ]]; then
  echo "All links are valid."
else
  echo "Link check completed with issues (exit code: $EXIT_CODE)."
  exit "$EXIT_CODE"
fi
