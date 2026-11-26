#!/usr/bin/env bash
# Checks that krit_aspect's persistent worker, whose one krit-fir JVM serves
# every target (#1666), reports exactly what one JVM per action does.
#
# Usage: scripts/bazel-worker-parity.sh [extra bazel args...]
#
# Generates chained kt_jvm_library targets under bazel/e2e/parity_generated/
# (removed afterwards) whose files share names and declarations but not
# types, with FIR-only findings that depend on the previous target's jar,
# builds their Krit actions with --@krit//:worker=false and =true, and diffs
# every target's report.
#
# Env: as scripts/bazel-worker-bench.sh (BAZEL, KRIT_BAZEL_LOCAL_DIST).
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
BAZEL="${BAZEL:-bazel}"
DIST="${KRIT_BAZEL_LOCAL_DIST:-${DIST:-${XDG_CACHE_HOME:-$HOME/.cache}/krit/bazel-e2e-dist}}"
if [ ! -x "$DIST/krit" ] || [ ! -f "$DIST/krit-fir.jar" ]; then
  echo "ERROR: $DIST has no krit and krit-fir.jar; run scripts/bazel-e2e-local.sh first" >&2
  exit 1
fi
export KRIT_BAZEL_LOCAL_DIST="$DIST"
e2e="$repo_root/bazel/e2e"
gen="$e2e/parity_generated"; out=$(mktemp -d)
trap 'rm -rf "$gen" "$out"' EXIT
rm -rf "$gen"
n=8
for i in $(seq 1 $n); do
  mkdir -p "$gen/t$i"
  dep=""
  if [ "$i" -gt 1 ]; then
    dep="    deps = [\"//parity_generated/t$((i - 1))\"],"
  fi
  printf 'load("@rules_kotlin//kotlin:jvm.bzl", "kt_jvm_library")\n\nkt_jvm_library(\n    name = "t%s",\n    srcs = glob(["*.kt"]),\n    visibility = ["//visibility:public"],\n%s\n)\n' "$i" "$dep" > "$gen/t$i/BUILD.bazel"
  # Same file and declaration names in every target, different types: a leaked session or
  # verdict would report the wrong finding set.
  ty="String?"
  if [ $((i % 2)) = 0 ]; then
    ty="String"
  fi
  cat > "$gen/t$i/Shared.kt" << KT
package parity.t$i

class Holder(val name: $ty)

fun len(h: Holder): Int? = h.name?.length
KT
  if [ "$i" -gt 1 ]; then
    printf 'package parity.t%s\n\nfun up(h: parity.t%s.Holder): Int? = h.name?.length\n' "$i" "$((i - 1))" > "$gen/t$i/Up.kt"
  fi
done
cd "$e2e"
# shellcheck disable=SC2086
build() {
  $BAZEL build --lockfile_mode=off --noshow_progress --symlink_prefix=/ --aspects=@krit//:defs.bzl%krit_aspect \
    --output_groups=krit "${extra[@]}" "$@" //parity_generated/... > "$out/build.log" 2>&1 || {
    cat "$out/build.log"
    exit 1
  }
}
extra=("$@")
# shellcheck disable=SC2086
bin="$($BAZEL info --lockfile_mode=off "${extra[@]}" bazel-bin 2> /dev/null)"
for mode in false true; do
  build --@krit//:worker=$mode
  mkdir -p "$out/$mode"
  for i in $(seq 1 $n); do cp "$bin/parity_generated/t$i/t$i.krit.txt" "$out/$mode/t$i.txt"; done
done
if diff -r "$out/false" "$out/true"; then
  echo "OK: $n targets, identical reports one-shot and on the shared worker JVM; UnnecessarySafeCall findings: $(cat "$out/true"/*.txt | grep -c UnnecessarySafeCall)"
else
  echo "ERROR: the shared worker JVM changed results"; exit 1
fi
