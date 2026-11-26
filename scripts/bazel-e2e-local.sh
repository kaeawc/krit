#!/usr/bin/env bash
# End-to-end check of the krit Bazel module (bazel/) against the analyzer
# built from this checkout: krit from cmd/krit and the krit-fir jar from
# tools/krit-fir, handed to the module through KRIT_BAZEL_LOCAL_DIST instead
# of a release download. Runs `bazel test //...` in bazel/e2e twice with a
# disk cache, and `bazel clean` in between; the second run must be a full
# cache hit (every test "(cached)"), which checks that the krit actions and
# tests are deterministic and carry no machine-specific inputs, so a remote
# cache can share them. Then builds krit_aspect's output group over
# //typed/... the same way (every Krit action of the second build must be a
# cache hit), and checks that --@krit//:fail_on_findings fails the build
# through a validation action on a target with findings, and only there,
# and that the rules_kotlin settings (kotlinc_opts, compiler plugins) reach
# krit-fir: without them //kotlin_settings's sources do not compile.
#
# Usage: scripts/bazel-e2e-local.sh [extra bazel test args...]
#
# Optional env:
#   KRIT_FIR_JAR  krit-fir jar to use (default: `make fir-jar`, which builds
#                 it once per source hash)
#   BAZEL         Bazel launcher (default bazel; bazelisk honours
#                 bazel/e2e/.bazelversion and USE_BAZEL_VERSION)
#   KRIT_E2E_ANDROID  set to 1 to also test bazel/e2e_android (rules_android
#                 targets; needs ANDROID_HOME with platform android-35 and
#                 build-tools 35.0.0)
#   DIST          directory for the local dist; Bazel's krit repositories
#                 link to its files, so it outlives the run (default:
#                 ~/.cache/krit/bazel-e2e-dist)

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
BAZEL="${BAZEL:-bazel}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
DIST="${DIST:-${XDG_CACHE_HOME:-$HOME/.cache}/krit/bazel-e2e-dist}"
mkdir -p "$DIST"
DIST="$(cd "$DIST" && pwd)"
disk_cache="$work/disk-cache"

cd "$repo_root"
# A shallow clone has no tags to describe; krit --version still needs to
# read as a version.
version="$(git describe --tags --always 2> /dev/null || true)"
case "$version" in
  *[0-9].[0-9]*) ;;
  *) version="0.0.0-dev+${version:-unknown}" ;;
esac
go build -ldflags "-s -w -X main.version=${version}" -o "$DIST/krit" ./cmd/krit/
jar="${KRIT_FIR_JAR:-}"
if [ -z "$jar" ]; then
  jar="$(bash scripts/build-dev-jar.sh krit-fir | tail -n 1)"
fi
if [ ! -f "$jar" ]; then
  echo "ERROR: krit-fir jar not found: $jar" >&2
  exit 1
fi
cp "$jar" "$DIST/krit-fir.jar"

cd "$repo_root/bazel/e2e"
export KRIT_BAZEL_LOCAL_DIST="$DIST"
run() {
  # BAZEL may carry startup options ("bazel --output_user_root=..."), so it
  # is split into words on purpose.
  # shellcheck disable=SC2086
  env -u KRIT_BAZEL_RELEASE -u KRIT_FIR_JAR $BAZEL test \
    --lockfile_mode=off \
    --test_output=errors \
    --noshow_progress \
    --symlink_prefix=/ \
    --disk_cache="$disk_cache" \
    "$@" //...
}

extra=("$@")
echo "== bazel test (first run)"
run "$@"

# shellcheck disable=SC2086
$BAZEL clean
echo "== bazel test (second run after bazel clean: every test must be cached)"
out="$(run "$@" 2>&1)" || {
  printf '%s\n' "$out"
  exit 1
}
printf '%s\n' "$out"
summary="$(printf '%s\n' "$out" | grep -E '^//[^ ]+ +.*(PASSED|FAILED|NO STATUS)' || true)"
if [ -z "$summary" ]; then
  echo "ERROR: no test summary in the second run" >&2
  exit 1
fi
if uncached="$(printf '%s\n' "$summary" | grep -v '(cached)')"; then
  echo "ERROR: the second run re-ran tests that should have been cache hits:" >&2
  printf '%s\n' "$uncached" >&2
  exit 1
fi
echo "OK: $(printf '%s\n' "$summary" | wc -l | tr -d ' ') tests, all cached on the second run"

build() {
  # shellcheck disable=SC2086
  env -u KRIT_BAZEL_RELEASE -u KRIT_FIR_JAR $BAZEL build \
    --lockfile_mode=off \
    --noshow_progress \
    --symlink_prefix=/ \
    --disk_cache="$disk_cache" \
    --aspects=@krit//:defs.bzl%krit_aspect \
    --output_groups=krit \
    "$@"
}

echo "== krit_aspect over //typed/... //kotlin_settings/... (first build)"
build "${extra[@]}" //typed/... //kotlin_settings/...
# shellcheck disable=SC2086
$BAZEL clean
echo "== krit_aspect over //typed/... //kotlin_settings/... (second build after bazel clean: every Krit action must be a cache hit)"
log="$work/exec.json"
build "${extra[@]}" --execution_log_json_file="$log" //typed/... //kotlin_settings/...
python3 - "$log" << 'PY'
import json, sys
text = open(sys.argv[1]).read()
decoder, pos, krit = json.JSONDecoder(), 0, []
while pos < len(text):
    while pos < len(text) and text[pos].isspace():
        pos += 1
    if pos >= len(text):
        break
    entry, pos = decoder.raw_decode(text, pos)
    if entry.get("mnemonic") == "Krit":
        krit.append(entry)
if not krit:
    sys.exit("ERROR: the execution log has no Krit actions")
missed = [e.get("targetLabel", "?") for e in krit if not (e.get("cacheHit") or e.get("remoteCacheHit"))]
if missed:
    sys.exit("ERROR: Krit actions re-ran instead of hitting the cache: " + ", ".join(missed))
print("OK: %d Krit actions, all cache hits after bazel clean" % len(krit))
PY

echo "== --@krit//:fail_on_findings fails the build on //typed/app's finding"
if out="$(build "${extra[@]}" --@krit//:fail_on_findings //typed/app 2>&1)"; then
  printf '%s\n' "$out"
  echo "ERROR: the build passed although //typed/app has a krit finding" >&2
  exit 1
fi
case "$out" in
  *"krit reported findings in //typed/app:app"*UnnecessarySafeCall*) ;;
  *)
    printf '%s\n' "$out"
    echo "ERROR: the build failed without the krit validation's report" >&2
    exit 1
    ;;
esac
echo "== --@krit//:fail_on_findings passes on the clean //typed/lib"
build "${extra[@]}" --@krit//:fail_on_findings //typed/lib

# Without the target's kotlinc_opts or compiler plugin the sources do not
# compile, so the Kotlin settings are what makes the FIR pass work.
for case in "settings_without_kotlinc_opts:'when' expression must be exhaustive" \
  "settings_without_plugin:'val' cannot be reassigned"; do
  target="//kotlin_settings:${case%%:*}"
  want="${case#*:}"
  echo "== $target: the Krit action fails without the Kotlin setting"
  if out="$(build "${extra[@]}" "$target" 2>&1)"; then
    printf '%s\n' "$out"
    echo "ERROR: krit analyzed $target although it lacks the Kotlin setting it needs" >&2
    exit 1
  fi
  case "$out" in
    *"$want"*) ;;
    *)
      printf '%s\n' "$out"
      echo "ERROR: $target failed for another reason than: $want" >&2
      exit 1
      ;;
  esac
done
echo "OK: krit_aspect"

# Persistent worker: the Krit actions above ran on a multiplex worker that
# keeps one krit-fir JVM warm. It must still analyze each request from its
# own inputs only: fix //typed/app's finding, and the warm worker's report
# must lose it; restore it, and the report must have it again.
echo "== persistent worker: Krit actions run on the worker and see each edit"
consumer="$repo_root/bazel/e2e/typed/app/Consumer.kt"
cp "$consumer" "$work/Consumer.kt.orig"
restore_consumer() { cp "$work/Consumer.kt.orig" "$consumer"; }
trap 'restore_consumer; rm -rf "$work"' EXIT
report() {
  build "${extra[@]}" --execution_log_json_file="$log" //typed/app > /dev/null 2>&1 || {
    build "${extra[@]}" //typed/app
    return 1
  }
  # shellcheck disable=SC2086
  cat "$($BAZEL info "${extra[@]}" bazel-bin 2> /dev/null)/typed/app/app.krit.txt"
}
sed -i.bak 's/upstream.name?.length/upstream.name.length/' "$consumer" && rm -f "$consumer.bak"
if report | grep -q UnnecessarySafeCall; then
  echo "ERROR: the worker reported a finding the edited source no longer has" >&2
  exit 1
fi
# The edited source is new to every cache, so this Krit action executed.
cp "$log" "$work/edited-exec.json"
restore_consumer
report | grep -q "typed/app/Consumer.kt:5:57: .*UnnecessarySafeCall" || {
  echo "ERROR: the worker lost //typed/app's finding after the source was restored" >&2
  exit 1
}
trap 'rm -rf "$work"' EXIT
python3 - "$work/edited-exec.json" << 'PY'
import json, sys
text = open(sys.argv[1]).read()
decoder, pos, runners = json.JSONDecoder(), 0, []
while pos < len(text):
    while pos < len(text) and text[pos].isspace():
        pos += 1
    if pos >= len(text):
        break
    entry, pos = decoder.raw_decode(text, pos)
    if entry.get("mnemonic") == "Krit":
        runners.append(entry.get("runner", ""))
if not runners or any("worker" not in r for r in runners):
    sys.exit("ERROR: Krit actions did not run on the persistent worker: %s" % runners)
print("OK: persistent worker (%s)" % ", ".join(runners))
PY

# export_model: a krit model of the Bazel graph for scans outside Bazel. A
# plain krit run in the workspace then finds .krit/bazel-model and gets the
# targets' classpath and Kotlin settings: the FIR-only findings of //typed/app
# (a type from //typed/lib's jar) and //kotlin_settings (kotlinc_opts and the
# assignment plugin) need both.
echo "== bazel run @krit//:export_model, then krit outside Bazel"
# shellcheck disable=SC2086
BAZEL="$BAZEL" $BAZEL run --lockfile_mode=off --noshow_progress --symlink_prefix=/ \
  --disk_cache="$disk_cache" "${extra[@]}" @krit//:export_model -- \
  --lockfile_mode=off --symlink_prefix=/ --disk_cache="$disk_cache" "${extra[@]}" \
  //typed/... //kotlin_settings/...
models=(.krit/bazel-model/*.krit-model.json)
[ -f "${models[0]}" ] || {
  echo "ERROR: export_model wrote no .krit/bazel-model/*.krit-model.json" >&2
  exit 1
}
if grep -l '@@WORKSPACE@@\|@@EXECROOT@@' "${models[@]}"; then
  echo "ERROR: export_model left path placeholders in the models above" >&2
  exit 1
fi
scan_home="$work/scan-home"
mkdir -p "$scan_home"
rc=0
scan="$(env -u KRIT_BAZEL_LOCAL_DIST HOME="$scan_home" KRIT_FIR_JAR="$DIST/krit-fir.jar" \
  KRIT_NO_JAR_DOWNLOAD=1 KRIT_NO_DAEMON_AUTOSTART=1 \
  "$DIST/krit" --no-daemon --no-cache --no-fir-daemon --fail-on-degraded -f plain --base-path . \
  typed/app/Consumer.kt kotlin_settings/Settings.kt 2>&1)" || rc=$?
printf '%s\n' "$scan"
if [ "$rc" != 1 ]; then
  echo "ERROR: krit exited $rc; want 1 (findings)" >&2
  exit 1
fi
for want in "typed/app/Consumer.kt:5:57:" "kotlin_settings/Settings.kt:32:"; do
  case "$scan" in
    *"$want"*UnnecessarySafeCall*) ;;
    *)
      echo "ERROR: krit with the exported model did not report UnnecessarySafeCall at $want" >&2
      exit 1
      ;;
  esac
done
rm -rf .krit/bazel-model
echo "OK: export_model"

if [ "${KRIT_E2E_ANDROID:-}" = 1 ]; then
  if [ -z "${ANDROID_HOME:-}" ]; then
    echo "ERROR: KRIT_E2E_ANDROID=1 needs ANDROID_HOME" >&2
    exit 1
  fi
  cd "$repo_root/bazel/e2e_android"
  echo "== bazel/e2e_android: bazel test (first run)"
  run "${extra[@]}"
  # shellcheck disable=SC2086
  $BAZEL clean
  echo "== bazel/e2e_android: bazel test (second run after bazel clean: every test must be cached)"
  out="$(run "${extra[@]}" 2>&1)" || {
    printf '%s\n' "$out"
    exit 1
  }
  printf '%s\n' "$out"
  if printf '%s\n' "$out" | grep -E '^//[^ ]+ +.*(PASSED|FAILED|NO STATUS)' | grep -v '(cached)'; then
    echo "ERROR: bazel/e2e_android re-ran tests that should have been cache hits" >&2
    exit 1
  fi
  echo "OK: bazel/e2e_android"
fi
