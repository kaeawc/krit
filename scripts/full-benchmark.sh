#!/usr/bin/env bash
set -euo pipefail

# Full benchmark suite across all available Kotlin codebases
# Usage: scripts/full-benchmark.sh

echo "=== Krit Full Benchmark Suite ==="
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Version: $(./krit --version 2>&1)"
echo "Platform: $(uname -s) $(uname -m)"
echo ""

PROJECTS=(
    "playground/kotlin-webservice:Playground Web Service"
    "playground/android-app:Playground Android App"
)

# Add local codebases if they exist
GITHUB_DIR="${GITHUB_DIR:-$HOME/github}"
for p in \
    "$GITHUB_DIR/nowinandroid:Now in Android (Google)" \
    "$GITHUB_DIR/coil:Coil (image loading)" \
    "$GITHUB_DIR/circuit:Circuit (Slack)" \
    "$GITHUB_DIR/anvil:Anvil (Square)" \
    "$GITHUB_DIR/detekt:detekt (linter)" \
    "$GITHUB_DIR/sentry-java:Sentry Java" \
    "$GITHUB_DIR/metro:Metro (Slack)" \
    "$GITHUB_DIR/apps-android-wikipedia:Wikipedia Android" \
    "$GITHUB_DIR/dd-sdk-android:Datadog Android SDK" \
    "$GITHUB_DIR/Signal-Android:Signal-Android"; do
    path="${p%%:*}"
    if [ -d "$path" ]; then
        PROJECTS+=("$p")
    fi
done

printf "| %-30s | %6s | %8s | %5s | %8s |\n" "Codebase" "Files" "Findings" "Rules" "Time"
printf "|%s|%s|%s|%s|%s|\n" "--------------------------------" "--------" "----------" "-------" "----------"

for entry in "${PROJECTS[@]}"; do
    path="${entry%%:*}"
    name="${entry#*:}"
    
    kt_files=$(find "$path" -name "*.kt" -not -path "*/build/*" 2>/dev/null | wc -l | tr -d ' ')
    
    start=$(python3 -c "import time;print(time.time())")
    result=$(./krit -f json -no-cache -no-type-inference -no-type-oracle -q "$path/" 2>/dev/null || true)
    end=$(python3 -c "import time;print(time.time())")
    duration=$(python3 -c "print(f'{$end-$start:.2f}s')")
    
    findings=$(echo "$result" | python3 -c "import json,sys;print(len(json.load(sys.stdin).get('findings',[])))" 2>/dev/null || echo "?")
    rules=$(echo "$result" | python3 -c "import json,sys;print(len(set(x['rule'] for x in json.load(sys.stdin).get('findings',[]))))" 2>/dev/null || echo "?")
    
    printf "| %-30s | %6s | %8s | %5s | %8s |\n" "$name" "$kt_files" "$findings" "$rules" "$duration"
done

echo ""
echo "All benchmarks: cold start, no cache, no type oracle."
