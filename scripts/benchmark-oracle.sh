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
TMPOUT=$(mktemp /tmp/krit-oracle-bench-XXXXXX.json)
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
    python3 << PYEOF
import json, sys

with open("${json_file}") as f:
    d = json.load(f)

total_ms = d.get("durationMs", 0)
findings = len(d.get("findings", []))
rules_triggered = len({x["rule"] for x in d.get("findings", [])})

def find(entries, *path):
    """Recursively find a named entry in the timing tree."""
    for e in (entries or []):
        if e.get("name") == path[0]:
            return find(e.get("children", []), *path[1:]) if len(path) > 1 else e
    return None

timings = d.get("perfTiming", [])

def ms(name, *path):
    e = find(timings, name, *path)
    return e.get("durationMs", 0) if e else 0

oracle_ms      = ms("typeOracle")
jvm_ms         = ms("typeOracle", "jvmAnalyze")
process_ms     = ms("typeOracle", "jvmAnalyze", "kritTypesProcess")
build_ms       = ms("typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinBuildSession")
analyze_ms     = ms("typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinAnalyzeFiles")
json_build_ms  = ms("typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinOracleJsonBuild")
parse_ms       = ms("parse")
type_idx_ms    = ms("typeIndex")
rule_exec_ms   = ms("ruleExecution")

def find_metrics(entries, *path):
    e = find(entries, *path)
    return e.get("metrics", {}) if e else {}

filter_m   = find_metrics(timings, "typeOracle", "jvmAnalyze", "oracleFilterSummary")
call_m     = find_metrics(timings, "typeOracle", "jvmAnalyze", "oracleCallFilterSummary")
analyze_m  = find_metrics(timings, "typeOracle", "jvmAnalyze", "kotlinTimings", "kotlinAnalyzeSummary")
rss_m      = find_metrics(timings, "typeOracle", "jvmAnalyze", "kritTypesProcessResources")

print(f"total_ms={total_ms}")
print(f"oracle_ms={oracle_ms}")
print(f"jvm_ms={jvm_ms}")
print(f"process_ms={process_ms}")
print(f"build_ms={build_ms}")
print(f"analyze_ms={analyze_ms}")
print(f"json_build_ms={json_build_ms}")
print(f"parse_ms={parse_ms}")
print(f"type_idx_ms={type_idx_ms}")
print(f"rule_exec_ms={rule_exec_ms}")
print(f"findings={findings}")
print(f"rules={rules_triggered}")
print(f"filter_files={filter_m.get('markedFiles', '?')}/{filter_m.get('totalFiles', '?')}")
print(f"callee_names={call_m.get('calleeNames', '?')}")
print(f"lexical_hints={call_m.get('lexicalHints', '?')}")
print(f"lexical_skips={call_m.get('lexicalSkips', '?')}")
print(f"kt_files_analyzed={analyze_m.get('files', '?')}")
print(f"peak_rss_mb={rss_m.get('peakRSSMB', '?')}")
PYEOF
}

for i in $(seq 1 "$RUNS"); do
    echo "--- Run $i/$RUNS ---"

    # Delete cached oracle to force a cold JVM invocation
    if [ -f "$ORACLE_JSON" ]; then
        rm -f "$ORACLE_JSON"
        echo "  Deleted cached oracle: $ORACLE_JSON"
    fi

    START_TS=$(python3 -c "import time; print(int(time.time()*1000))")
    "$KRIT" -no-cache -no-cache-oracle -perf -f json -q "$PROJECT/" \
        > "$TMPOUT" 2>/dev/null || true
    END_TS=$(python3 -c "import time; print(int(time.time()*1000))")
    WALL_MS=$(( END_TS - START_TS ))

    # Parse timing from JSON output into a sourceable temp file
    PARSED_VARS=$(mktemp /tmp/krit-bench-vars-XXXXXX.sh)
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
