#!/usr/bin/env bash
set -euo pipefail

# Cold KAA oracle benchmark for krit-types JVM analysis.
#
# Measures how long krit-types.jar takes on a Kotlin project with no cached
# oracle output. Deletes .krit/types.json before each run so every run is
# truly cold (no oracle cache warm-up).
#
# Usage:
#   scripts/benchmark-oracle.sh /path/to/kotlin/project [runs]
#
# Arguments:
#   PROJECT   Path to the Kotlin project to analyze (default: .)
#   RUNS      Number of cold runs (default: 2)
#
# Output:
#   Per-run breakdown: total wall, jvmAnalyze, kritTypesProcess,
#   kotlinBuildSession, kotlinAnalyzeFiles, plus non-oracle phases.
#   After all runs, a summary table.

PROJECT="${1:-.}"
RUNS="${2:-2}"
ORACLE_JSON="${PROJECT}/.krit/types.json"
KRIT="${KRIT:-./krit}"
TMPOUT="$(mktemp /tmp/krit-oracle-bench-XXXXXX).json"
trap 'rm -f "$TMPOUT"' EXIT

if [ ! -d "$PROJECT" ]; then
    echo "error: project directory not found: $PROJECT" >&2
    exit 1
fi

# Verify binary exists
if [ ! -x "$KRIT" ]; then
    echo "error: krit binary not found at $KRIT — build with: go build -o krit ./cmd/krit/" >&2
    exit 1
fi

# Verify jar exists (oracle requires it)
JAR_FOUND=false
for candidate in \
        "tools/krit-types/build/libs/krit-types.jar" \
        "$(dirname "$KRIT")/tools/krit-types/build/libs/krit-types.jar" \
        "${PROJECT}/.krit/krit-types.jar"; do
    if [ -f "$candidate" ]; then JAR_FOUND=true; break; fi
done
if ! $JAR_FOUND; then
    echo "error: krit-types.jar not found — build with: cd tools/krit-types && ./gradlew shadowJar" >&2
    exit 1
fi

KT_FILES=$(find "$PROJECT" -name "*.kt" -not -path "*/build/*" 2>/dev/null | wc -l | tr -d ' ')

echo "=== Cold KAA Oracle Benchmark ==="
echo "  Project:      $PROJECT"
echo "  Kotlin files: $KT_FILES"
echo "  Runs:         $RUNS"
echo "  Binary:       $KRIT"
echo ""

# Arrays to collect per-run numbers for the summary table
WALLS=()
JVM_TOTALS=()
KT_PROCESS=()
KT_BUILD=()
KT_ANALYZE=()
FINDINGS=()
RULES=()

parse_timing() {
    local json_file="$1"
    go run ./internal/devtools/jsonstat -mode oracle-bench-env -file "$json_file"
}

for i in $(seq 1 "$RUNS"); do
    echo "--- Run $i/$RUNS ---"

    # Delete cached oracle to force a cold JVM invocation
    if [ -f "$ORACLE_JSON" ]; then
        rm -f "$ORACLE_JSON"
        echo "  Deleted cached oracle: $ORACLE_JSON"
    fi

    START_TS=$(go run ./internal/devtools/jsonstat -mode unix-ms)
    "$KRIT" -no-cache -no-cache-oracle -perf -f json -q "$PROJECT/" \
        > "$TMPOUT" 2>/dev/null || true
    END_TS=$(go run ./internal/devtools/jsonstat -mode unix-ms)
    WALL_MS=$(( END_TS - START_TS ))

    # Parse timing from JSON output into a sourceable temp file
    PARSED_VARS="$(mktemp /tmp/krit-bench-vars-XXXXXX).sh"
    parse_timing "$TMPOUT" | sed 's/^/export /' > "$PARSED_VARS"
    # shellcheck disable=SC1090
    source "$PARSED_VARS"
    rm -f "$PARSED_VARS"

    echo "  Wall clock:           ${WALL_MS}ms"
    echo "  durationMs (JSON):    ${total_ms}ms"
    echo ""
    echo "  typeOracle:           ${oracle_ms}ms"
    echo "    jvmAnalyze:         ${jvm_ms}ms"
    echo "      kritTypesProcess: ${process_ms}ms"
    echo "      kotlinBuildSession: ${build_ms}ms"
    echo "      kotlinAnalyzeFiles: ${analyze_ms}ms  ← main KAA phase"
    echo "      kotlinOracleJsonBuild: ${json_build_ms}ms"
    echo ""
    echo "  parse:                ${parse_ms}ms"
    echo "  typeIndex:            ${type_idx_ms}ms"
    echo "  ruleExecution:        ${rule_exec_ms}ms"
    echo ""
    echo "  Findings:             ${findings}  Rules triggered: ${rules}"
    echo "  Oracle filter:        ${filter_files} files"
    echo "  Call filter:          ${callee_names} callee names, ${lexical_hints} hints, ${lexical_skips} skips"
    echo "  KAA files analyzed:   ${kt_files_analyzed}"
    echo "  Peak RSS:             ${peak_rss_mb} MB"
    echo ""

    WALLS+=("$total_ms")
    JVM_TOTALS+=("$jvm_ms")
    KT_PROCESS+=("$process_ms")
    KT_BUILD+=("$build_ms")
    KT_ANALYZE+=("$analyze_ms")
    FINDINGS+=("$findings")
    RULES+=("$rules")
done

# Summary table
echo "=== Summary ==="
printf "%-8s %10s %10s %10s %10s %12s %10s %8s\n" \
    "Run" "Total" "jvmAnalyze" "JVMProcess" "ktBuild" "ktAnalyze" "Findings" "Rules"
printf "%-8s %10s %10s %10s %10s %12s %10s %8s\n" \
    "---" "-----" "----------" "----------" "-------" "---------" "--------" "-----"
for i in $(seq 0 $(( RUNS - 1 ))); do
    printf "%-8s %9sms %9sms %9sms %9sms %11sms %10s %8s\n" \
        "$((i+1))" \
        "${WALLS[$i]}" \
        "${JVM_TOTALS[$i]}" \
        "${KT_PROCESS[$i]}" \
        "${KT_BUILD[$i]}" \
        "${KT_ANALYZE[$i]}" \
        "${FINDINGS[$i]}" \
        "${RULES[$i]}"
done
echo ""
echo "Key metric: kotlinAnalyzeFiles (the KAA declaration extraction phase)"
echo "Target from issue #420: reduce by ≥20% for narrow declaration profiles"
