#!/usr/bin/env bash
set -euo pipefail

# Full benchmark suite across playground projects and optional user-supplied targets.
# Usage:
#   scripts/full-benchmark.sh
#   EXTRA_BENCHMARK_PROJECTS="/path/to/repo:Display Name;/path/to/other:Other" scripts/full-benchmark.sh

echo "=== Krit Full Benchmark Suite ==="
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Version: $(./krit --version 2>&1)"
echo "Platform: $(uname -s) $(uname -m)"
echo ""

PROJECTS=(
    "playground/kotlin-webservice:Playground Web Service"
    "playground/android-app:Playground Android App"
)

# Add caller-provided local codebases if they exist.
IFS=';' read -r -a EXTRA_PROJECTS <<< "${EXTRA_BENCHMARK_PROJECTS:-}"
for p in "${EXTRA_PROJECTS[@]}"; do
    [ -n "$p" ] || continue
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
    
    source_files=$(find "$path" \( -name "*.kt" -o -name "*.java" -o -name "*.xml" -o -name "*.gradle" -o -name "*.gradle.kts" \) -not -path "*/build/*" 2>/dev/null | wc -l | tr -d ' ')
    
    start=$(go run ./internal/devtools/jsonstat -mode unix-ms)
    result=$(./krit -f json -no-cache -no-type-inference -no-type-oracle -q "$path/" 2>/dev/null || true)
    end=$(go run ./internal/devtools/jsonstat -mode unix-ms)
    duration="$((end - start))ms"

    findings=$(echo "$result" | go run ./internal/devtools/jsonstat -mode findings 2>/dev/null || echo "?")
    rules=$(echo "$result" | go run ./internal/devtools/jsonstat -mode rules 2>/dev/null || echo "?")
    
    printf "| %-30s | %6s | %8s | %5s | %8s |\n" "$name" "$source_files" "$findings" "$rules" "$duration"
done

echo ""
echo "All benchmarks: cold start, no cache, no type oracle."
