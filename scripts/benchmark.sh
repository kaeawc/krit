#!/usr/bin/env bash
set -euo pipefail

# Benchmark krit on a Kotlin project
# Usage: scripts/benchmark.sh /path/to/kotlin/project [config.yml]

PROJECT="${1:-.}"
CONFIG="${2:-}"

if [ ! -d "$PROJECT" ]; then
    echo "Usage: scripts/benchmark.sh /path/to/kotlin/project [config.yml]"
    exit 1
fi

KT_FILES=$(find "$PROJECT" -name "*.kt" -not -path "*/build/*" 2>/dev/null | wc -l | tr -d ' ')

echo "=== Krit Benchmark ==="
echo "  Project: $PROJECT"
echo "  Kotlin files: $KT_FILES"
echo ""

ARGS="-f json -no-cache -no-type-inference -no-type-oracle -q"
if [ -n "$CONFIG" ]; then
    ARGS="$ARGS -config $CONFIG"
fi

echo "Running..."
TIMEFORMAT='  Wall time: %Rs'
time {
    result=$(./krit $ARGS "$PROJECT/" 2>/dev/null || true)
    findings=$(echo "$result" | go run ./internal/devtools/jsonstat -mode findings 2>/dev/null || echo "?")
    source_findings=$(echo "$result" | go run ./internal/devtools/jsonstat -mode source-findings 2>/dev/null || echo "?")
    rules=$(echo "$result" | go run ./internal/devtools/jsonstat -mode rules 2>/dev/null || echo "?")
    echo "  Findings: $findings ($source_findings in source)"
    echo "  Rules triggered: $rules"
}
