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
    ./krit $ARGS "$PROJECT/" 2>/dev/null | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    f = d.get('findings', [])
    rules = {}
    for x in f:
        rules[x['rule']] = rules.get(x['rule'], 0) + 1
    src = [x for x in f if '/test/' not in x['file']]
    print(f'  Findings: {len(f)} ({len(src)} in source)')
    print(f'  Rules triggered: {len(rules)}')
    print()
    print('  Top 5 issues:')
    for r, c in sorted(rules.items(), key=lambda x: -x[1])[:5]:
        print(f'    {r}: {c}')
except:
    pass
" || true
}
