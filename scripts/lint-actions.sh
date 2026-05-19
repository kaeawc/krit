#!/usr/bin/env bash
# Lint the GitHub Actions workflow files with actionlint.
#
# actionlint catches semantic errors that plain YAML parsers (including
# yamllint) accept — most importantly, function-call context violations
# like `${{ hashFiles(...) }}` at workflow-`env` scope, where it is
# syntactically valid YAML but illegal in GitHub Actions and causes the
# workflow to fail to load with zero checks scheduled.
#
# Run this after touching any file under `.github/workflows/`. CI runs
# the same command in the `actionlint` job so missing a local check
# just costs a round-trip.

set -euo pipefail

if ! command -v actionlint >/dev/null 2>&1; then
    cat >&2 <<'EOF'
error: actionlint is not on PATH.

Install with one of:
  brew install actionlint
  go install github.com/rhysd/actionlint/cmd/actionlint@latest

Then re-run this script.
EOF
    exit 127
fi

cd "$(dirname "$0")/.."

# `actionlint` exits 0 on success, non-zero on findings. Auto-discovery
# walks `.github/workflows/` (and composite actions under
# `.github/actions/`) without needing explicit args.
#
# Shellcheck severity is raised to `warning` so existing info-level
# findings in `release.yml` (a separate cleanup) don't gate every
# unrelated workflow edit. Genuine bugs land at warning+; SC2086 / SC2035
# infos can be addressed in a focused PR.
actionlint -shellcheck "shellcheck --severity=warning"
