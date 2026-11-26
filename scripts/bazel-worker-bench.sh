#!/usr/bin/env bash
# Benchmarks krit_aspect's Krit actions on a generated Bazel workspace:
# a one-shot krit-fir JVM per action (--@krit//:worker=false, as on remote
# execution) against the multiplex persistent worker, and both against one
# whole-repository `krit --fir` over the exported model.
#
# Usage: scripts/bazel-worker-bench.sh [TARGETS] [extra bazel args...]
#
# Generates TARGETS (default 24) kt_jvm_library targets under
# bazel/e2e/bench_generated/ (removed afterwards), each with two files and a
# dependency on the previous target, and prints a Markdown table.
#
# Env: as scripts/bazel-e2e-local.sh (KRIT_FIR_JAR, BAZEL, DIST); run that
# script once first, or set KRIT_BAZEL_LOCAL_DIST to a directory with krit
# and krit-fir.jar.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
targets="${1:-24}"
shift || true
BAZEL="${BAZEL:-bazel}"
DIST="${KRIT_BAZEL_LOCAL_DIST:-${DIST:-${XDG_CACHE_HOME:-$HOME/.cache}/krit/bazel-e2e-dist}}"
if [ ! -x "$DIST/krit" ] || [ ! -f "$DIST/krit-fir.jar" ]; then
  echo "ERROR: $DIST has no krit and krit-fir.jar; run scripts/bazel-e2e-local.sh first" >&2
  exit 1
fi
export KRIT_BAZEL_LOCAL_DIST="$DIST"

e2e="$repo_root/bazel/e2e"
gen="$e2e/bench_generated"
work="$(mktemp -d)"
trap 'rm -rf "$gen" "$work" "$e2e/.krit/bazel-model"' EXIT
rm -rf "$gen"
mkdir -p "$gen"

generate() {
  local salt="$1"
  for i in $(seq 1 "$targets"); do
    local dir="$gen/t$i"
    mkdir -p "$dir"
    local dep=""
    if [ "$i" -gt 1 ]; then
      dep="    deps = [\"//bench_generated/t$((i - 1))\"],"
    fi
    cat > "$dir/BUILD.bazel" << EOF
load("@rules_kotlin//kotlin:jvm.bzl", "kt_jvm_library")

kt_jvm_library(
    name = "t$i",
    srcs = glob(["*.kt"]),
    visibility = ["//visibility:public"],
$dep
)
EOF
    cat > "$dir/Model$i.kt" << EOF
package bench.t$i

// $salt
data class Model$i(val id: Int, val name: String, val tags: List<String>)

fun model$i(id: Int): Model$i = Model$i(id, "model-\$id", listOf("a", "b"))
EOF
    local use="fun use$i(): Int = model$i($i).name.length"
    if [ "$i" -gt 1 ]; then
      use="fun use$i(): Int = model$i($i).name.length + bench.t$((i - 1)).model$((i - 1))($i).tags.size"
    fi
    cat > "$dir/Use$i.kt" << EOF
package bench.t$i

$use

fun labels$i(models: List<Model$i>): Map<Int, String> = models.associate { it.id to it.name.uppercase() }
EOF
  done
}

cd "$e2e"
# shellcheck disable=SC2086
build() {
  $BAZEL build --lockfile_mode=off --noshow_progress --symlink_prefix=/ \
    --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit "$@" //bench_generated/...
}

krit_stats() {
  python3 - "$1" << 'PY'
import json, sys
text = open(sys.argv[1]).read()
decoder, pos, times = json.JSONDecoder(), 0, []
while pos < len(text):
    while pos < len(text) and text[pos].isspace():
        pos += 1
    if pos >= len(text):
        break
    entry, pos = decoder.raw_decode(text, pos)
    if entry.get("mnemonic") == "Krit" and not entry.get("cacheHit"):
        times.append(float(entry.get("metrics", {}).get("totalTime", "0s").rstrip("s")))
times.sort()
if not times:
    print("0 - - -")
else:
    print(len(times), "%.2f" % (sum(times) / len(times)), "%.2f" % times[len(times) // 2], "%.2f" % sum(times))
PY
}

now() { date +%s.%N; }
elapsed() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.1f", b - a }'; }

echo "== compiling $targets generated targets" >&2
generate base
build "$@" > /dev/null 2>&1 || build "$@"

rows=()
for mode in false true; do
  generate "$mode-$RANDOM"
  # The Kotlin compile is not what is measured: build it first.
  # shellcheck disable=SC2086
  $BAZEL build --lockfile_mode=off --noshow_progress --symlink_prefix=/ "$@" //bench_generated/... > /dev/null 2>&1
  start="$(now)"
  build --@krit//:worker="$mode" --execution_log_json_file="$work/$mode.json" "$@" > /dev/null 2>&1
  end="$(now)"
  read -r count mean median sum <<< "$(krit_stats "$work/$mode.json")"
  label="one-shot JVM per action (--@krit//:worker=false)"
  if [ "$mode" = true ]; then
    label="persistent worker"
  fi
  rows+=("| $label | $count | $mean s | $median s | $sum s | $(elapsed "$start" "$end") s |")
done

# The edit loop: change one target's own sources and rebuild; its classpath
# is unchanged, so a worker that analyzed it before has its JVM warm.
leaf="$gen/t$targets/Use$targets.kt"
edit_rows=()
for mode in false true; do
  times=()
  for round in 1 2 3 4; do
    echo "// edit $mode $round $RANDOM" >> "$leaf"
    # shellcheck disable=SC2086
    $BAZEL build --lockfile_mode=off --noshow_progress --symlink_prefix=/ "$@" "//bench_generated/t$targets" > /dev/null 2>&1
    build --@krit//:worker="$mode" --execution_log_json_file="$work/edit.json" "$@" > /dev/null 2>&1
    read -r _ mean _ _ <<< "$(krit_stats "$work/edit.json")"
    # The first round may start the worker's JVM for this target.
    [ "$round" = 1 ] || times+=("$mean")
  done
  label="one-shot JVM per action"
  if [ "$mode" = true ]; then
    label="persistent worker"
  fi
  edit_rows+=("| $label | $(printf '%s\n' "${times[@]}" | awk '{ s += $1 } END { printf "%.2f", s / NR }') s |")
done

# One whole-repository krit --fir over the exported model.
# shellcheck disable=SC2086
BAZEL="$BAZEL" $BAZEL run --lockfile_mode=off --noshow_progress --symlink_prefix=/ "$@" @krit//:export_model -- \
  --lockfile_mode=off --symlink_prefix=/ "$@" //bench_generated/... > /dev/null 2>&1
home="$work/home"
mkdir -p "$home"
start="$(now)"
HOME="$home" KRIT_FIR_JAR="$DIST/krit-fir.jar" KRIT_NO_JAR_DOWNLOAD=1 KRIT_NO_DAEMON_AUTOSTART=1 \
  "$DIST/krit" --no-daemon --no-cache --no-fir-daemon --fail-on-degraded -f plain --base-path . bench_generated \
  > "$work/whole.txt" 2>&1 || {
  rc=$?
  # Exit 1 is findings; anything else (2: FIR could not run) is no result.
  if [ "$rc" != 1 ]; then
    cat "$work/whole.txt" >&2
    echo "ERROR: the whole-repository krit --fir scan failed (exit $rc)" >&2
    exit 1
  fi
}
end="$(now)"
whole="$(elapsed "$start" "$end")"

echo
echo "$targets generated kt_jvm_library targets (2 files each, chained deps); Krit actions only (Kotlin compile excluded):"
echo
echo "| Mode | Krit actions | Mean | Median | Sum | Wall time |"
echo "| --- | --- | --- | --- | --- | --- |"
printf '%s\n' "${rows[@]}"
echo "| whole-repo \`krit --fir\` (exported model, one cold JVM) | 1 scan | - | - | - | $whole s |"
echo
echo "Edit loop: one target's own source edited, then rebuilt (mean Krit action over rounds 2-4):"
echo
echo "| Mode | Krit action |"
echo "| --- | --- |"
printf '%s\n' "${edit_rows[@]}"
