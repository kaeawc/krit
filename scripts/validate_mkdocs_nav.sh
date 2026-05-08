#!/usr/bin/env bash
# Validate that all files referenced in mkdocs.yml nav exist in docs/.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MKDOCS="$ROOT/mkdocs.yml"
DOCS="$ROOT/docs"

if [[ ! -f "$MKDOCS" ]]; then
  echo "ERROR: mkdocs.yml not found at $MKDOCS"
  exit 1
fi

errors=0

# Extract .md file references from nav section of mkdocs.yml
while IFS= read -r file; do
  # Trim whitespace
  file="$(echo "$file" | xargs)"
  if [[ -z "$file" ]]; then
    continue
  fi
  if [[ ! -f "$DOCS/$file" ]]; then
    echo "MISSING: docs/$file (referenced in mkdocs.yml nav)"
    errors=$((errors + 1))
  fi
done < <(grep -oP ':\s+\K[^\s]+\.md' "$MKDOCS" 2>/dev/null || \
         grep -oE ':\s+[^ ]+\.md' "$MKDOCS" | sed 's/^:\s*//')

if [[ "$errors" -gt 0 ]]; then
  echo ""
  echo "Found $errors missing nav reference(s)."
  exit 1
else
  echo "All mkdocs.yml nav references are valid."
fi
