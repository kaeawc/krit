#!/usr/bin/env bash
set -euo pipefail

# Compare KAA vs FIR oracle backend wall-clock on a Kotlin project.
# Uses `hyperfine` when available and falls back to a bash timing
# loop. For the JVM-internal breakdown of one backend, use
# scripts/benchmark-oracle.sh instead.

usage() {
    cat <<'EOF'
Usage: scripts/bench-oracle-backend.sh PROJECT_DIR [options]

Options:
  --runs N      Measured runs per backend (default: 3)
  --warmup N    Unmeasured warmup runs per backend (default: 1)
  --cold        Delete oracle cache + krit cache before each run
  --krit PATH   Path to the krit binary (default: ./krit)
  --backend B   Limit to one backend (kaa|fir); default runs both
  -h, --help    Show this help and exit
EOF
}

PROJECT=""
RUNS=3
WARMUP=1
COLD=0
KRIT="./krit"
BACKENDS=("kaa" "fir")

while [ $# -gt 0 ]; do
    case "$1" in
        --runs) RUNS="$2"; shift 2 ;;
        --warmup) WARMUP="$2"; shift 2 ;;
        --cold) COLD=1; shift ;;
        --krit) KRIT="$2"; shift 2 ;;
        --backend) BACKENDS=("$2"); shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --) shift; break ;;
        -*) echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
        *) if [ -z "$PROJECT" ]; then PROJECT="$1"; shift; else
               echo "error: unexpected positional arg: $1" >&2; exit 2; fi ;;
    esac
done

if [ -z "$PROJECT" ]; then
    usage >&2
    exit 2
fi
if [ ! -d "$PROJECT" ]; then
    echo "error: project directory not found: $PROJECT" >&2
    exit 1
fi
if [ ! -x "$KRIT" ]; then
    echo "error: krit binary not found at $KRIT — build with: go build -o krit ./cmd/krit/" >&2
    exit 1
fi

require_jar() {
    local name="$1"
    for candidate in \
            "tools/${name%.jar}/build/libs/$name" \
            "$(dirname "$KRIT")/tools/${name%.jar}/build/libs/$name"; do
        if [ -f "$candidate" ]; then return 0; fi
    done
    echo "error: $name not found — build with: cd tools/${name%.jar} && ./gradlew shadowJar" >&2
    return 1
}
for backend in "${BACKENDS[@]}"; do
    case "$backend" in
        kaa) require_jar "krit-types.jar" ;;
        fir) require_jar "krit-fir.jar" ;;
        *) echo "error: unknown backend: $backend (want kaa or fir)" >&2; exit 2 ;;
    esac
done

SOURCE_FILES=$(find "$PROJECT" \( -name "*.kt" -o -name "*.java" -o -name "*.xml" -o -name "*.gradle" -o -name "*.gradle.kts" \) -not -path "*/build/*" 2>/dev/null | wc -l | tr -d ' ')

echo "=== KAA vs FIR Oracle Backend Benchmark ==="
echo "  Project:      $PROJECT"
echo "  Source files: $SOURCE_FILES"
echo "  Runs:         $RUNS (after $WARMUP warmup)"
echo "  Cold mode:    $([ "$COLD" = 1 ] && echo "yes — caches cleared per run" || echo "no — caches warm")"
echo "  Backends:     ${BACKENDS[*]}"
echo ""

COLD_CMD="rm -rf '$PROJECT/.krit/cache' '$PROJECT/.krit/types.json' '$PROJECT/.krit/fir.json' 2>/dev/null || true"

if command -v hyperfine >/dev/null 2>&1; then
    HYPER_ARGS=( --warmup "$WARMUP" --runs "$RUNS" )
    if [ "$COLD" = 1 ]; then
        HYPER_ARGS+=( --prepare "$COLD_CMD" )
    fi
    HYPER_ARGS+=( --export-json /tmp/krit-oracle-bench.json )
    CMDS=()
    for backend in "${BACKENDS[@]}"; do
        CMDS+=( --command-name "$backend" "$KRIT --oracle-backend=$backend -q -f json $PROJECT/" )
    done
    hyperfine "${HYPER_ARGS[@]}" "${CMDS[@]}"
    echo ""
    echo "Detail JSON: /tmp/krit-oracle-bench.json"
    exit 0
fi

echo "(hyperfine not on PATH; using bash timing loop)"
echo ""

# date +%N prints a literal "N" on macOS / BSD; fall back to python.
now_ms() {
    if [[ "$(date +%N)" == "N" ]]; then
        python3 -c 'import time; print(int(time.time()*1000))'
    else
        echo $(( $(date +%s%N) / 1000000 ))
    fi
}

run_once() {
    local backend="$1"
    if [ "$COLD" = 1 ]; then eval "$COLD_CMD"; fi
    "$KRIT" --oracle-backend="$backend" -q -f json "$PROJECT/" >/dev/null 2>&1 || true
}

for backend in "${BACKENDS[@]}"; do
    echo "--- ${backend^^} ---"
    for _ in $(seq 1 "$WARMUP"); do run_once "$backend"; done
    samples=()
    for i in $(seq 1 "$RUNS"); do
        t0=$(now_ms)
        run_once "$backend"
        t1=$(now_ms)
        samples+=("$(( t1 - t0 ))")
        printf "  run %d: %sms\n" "$i" "${samples[-1]}"
    done
    sorted=$(printf "%s\n" "${samples[@]}" | sort -n)
    median=$(echo "$sorted" | sed -n "$(( ${#samples[@]} / 2 + 1 ))p")
    echo "  median: ${median}ms"
    echo ""
done
