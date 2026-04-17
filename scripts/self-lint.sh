#!/usr/bin/env bash
set -euo pipefail

# Builds krit and runs it on itself, producing SARIF output.
# Used by the self-lint GitHub Actions workflow.

go build -o krit ./cmd/krit/
./krit --format sarif . > results.sarif || true
