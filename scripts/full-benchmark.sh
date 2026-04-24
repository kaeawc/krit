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
for p in \
    "/Users/jason/github/nowinandroid:Now in Android (Google)" \
    "/Users/jason/github/coil:Coil (image loading)" \
    "/Users/jason/github/circuit:Circuit (Slack)" \
    "/Users/jason/github/anvil:Anvil (Square)" \
    "/Users/jason/github/detekt:detekt (linter)" \
    "/Users/jason/github/sentry-java:Sentry Java" \
    "/Users/jason/github/metro:Metro (Slack)" \
    "/Users/jason/github/apps-android-wikipedia:Wikipedia Android" \
    "/Users/jason/github/dd-sdk-android:Datadog Android SDK" \
    "/Users/jason/github/Signal-Android:Signal-Android"; do
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
    
    start=$(go run ./internal/devtools/jsonstat -mode unix-ms)
    result=$(./krit -f json -no-cache -no-type-inference -no-type-oracle -q "$path/" 2>/dev/null || true)
    end=$(go run ./internal/devtools/jsonstat -mode unix-ms)
    duration="$((end - start))ms"

    findings=$(echo "$result" | go run ./internal/devtools/jsonstat -mode findings 2>/dev/null || echo "?")
    rules=$(echo "$result" | go run ./internal/devtools/jsonstat -mode rules 2>/dev/null || echo "?")
    
    printf "| %-30s | %6s | %8s | %5s | %8s |\n" "$name" "$kt_files" "$findings" "$rules" "$duration"
done

echo ""
echo "All benchmarks: cold start, no cache, no type oracle."
