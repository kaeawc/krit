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
# cache hit), checks that per-target actions leave project-scope rules to
# krit_project_report, and checks that --@krit//:fail_on_findings fails the build
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
# A target whose srcs include a generated srcjar (#1688, #1632): its own
# file, in a Gradle-style src/main/kotlin, compiles against the srcjar's
# sources, so the action is not degraded and FIR reports the finding.
echo "== krit_aspect on //srcjar (a generated srcjar in srcs)"
build "${extra[@]}" //srcjar/... > /dev/null 2>&1 || build "${extra[@]}" //srcjar/...
# shellcheck disable=SC2086
grep -q "srcjar/src/main/kotlin/d/UsesGen.kt:5:.*UnnecessarySafeCall" \
  "$($BAZEL info "${extra[@]}" bazel-bin 2> /dev/null)/srcjar/srcjar.krit.txt" || {
  # shellcheck disable=SC2086
  cat "$($BAZEL info "${extra[@]}" bazel-bin 2> /dev/null)/srcjar/srcjar.krit.txt" >&2
  echo "ERROR: krit_aspect did not report //srcjar's FIR finding through the srcjar" >&2
  exit 1
}
# A minSdk set only by the build (#1682): with --@krit//:min_sdk_version=24
# the SDK_INT >= 21 check of //sdk is obsolete; without it the level is
# unknown and krit reports nothing.
echo "== --@krit//:min_sdk_version reaches krit_aspect's actions"
sdk_report() {
  build "${extra[@]}" "$@" //sdk > /dev/null 2>&1 || build "${extra[@]}" "$@" //sdk
  # shellcheck disable=SC2086
  cat "$($BAZEL info "${extra[@]}" bazel-bin 2> /dev/null)/sdk/sdk.krit.txt"
}
if sdk_report | grep -q ObsoleteSdkInt; then
  echo "ERROR: //sdk reported ObsoleteSdkInt without a minSdk" >&2
  exit 1
fi
sdk_report --@krit//:min_sdk_version=24 | grep -q "sdk/Levels.kt:5:.*ObsoleteSdkInt.*always >= 24" || {
  sdk_report --@krit//:min_sdk_version=24 >&2
  echo "ERROR: --@krit//:min_sdk_version=24 did not reach //sdk's Krit action" >&2
  exit 1
}
echo "OK: krit_aspect"

# A remote cache shares a Krit action between machines only if its key
# names no machine-specific path: the command line, the params file, the
# environment and the model.json the action reads must all be relative to
# the execution root.
echo "== krit_aspect action keys carry no absolute paths"
# shellcheck disable=SC2086
env -u KRIT_BAZEL_RELEASE -u KRIT_FIR_JAR $BAZEL aquery \
  --lockfile_mode=off \
  --noshow_progress \
  --symlink_prefix=/ \
  --aspects=@krit//:defs.bzl%krit_aspect \
  --output_groups=krit \
  --include_param_files \
  --include_file_write_contents \
  --output=jsonproto \
  "${extra[@]}" 'mnemonic("^(Krit|FileWrite)$", //typed/... + //kotlin_settings/...)' > "$work/aquery.json"
python3 - "$work/aquery.json" << 'PY'
import json, re, sys
graph = json.load(open(sys.argv[1]))
# An absolute path at the start of a value or after "=", ":", "," or a quote;
# "//pkg:name" labels are not paths.
absolute = re.compile(r'(?:^|[=:,"\s])/(?!/)')
krit, models, bad = 0, 0, []
for action in graph.get("actions", []):
    values = []
    if action.get("mnemonic") == "Krit":
        krit += 1
        values += action.get("arguments", [])
        values += [v.get("value", "") for v in action.get("environmentVariables", [])]
        for params in action.get("paramFiles", []):
            values += params.get("arguments", [])
    elif '"sourceSets"' in action.get("fileContents", ""):
        models += 1
        values.append(action["fileContents"])
    for value in values:
        for line in value.splitlines():
            if absolute.search(line):
                bad.append(line.strip())
if not krit or not models:
    sys.exit("ERROR: aquery found %d Krit actions and %d models" % (krit, models))
if bad:
    sys.exit("ERROR: Krit action keys name absolute paths:\n  " + "\n  ".join(sorted(set(bad))))
print("OK: %d Krit actions and %d models, no absolute paths" % (krit, models))
PY

# Compile avoidance: with rules_kotlin's ABI jars an implementation-only
# edit of //typed/lib leaves its compile jar, and so //typed/app's Krit
# action key, unchanged; only //typed/lib's own Krit action re-runs.
echo "== an implementation-only edit of //typed/lib re-runs only its own Krit action"
upstream="$repo_root/bazel/e2e/typed/lib/Upstream.kt"
cp "$upstream" "$work/Upstream.kt.orig"
trap 'cp "$work/Upstream.kt.orig" "$upstream"; rm -rf "$work"' EXIT
abi=(--define=experimental_use_abi_jars=1)
build "${extra[@]}" "${abi[@]}" //typed/... > /dev/null 2>&1 || build "${extra[@]}" "${abi[@]}" //typed/...
sed -i.bak 's/"upstream"/"upstream, edited"/' "$upstream" && rm -f "$upstream.bak"
build "${extra[@]}" "${abi[@]}" --execution_log_json_file="$log" //typed/... > /dev/null 2>&1 ||
  build "${extra[@]}" "${abi[@]}" //typed/...
cp "$work/Upstream.kt.orig" "$upstream"
trap 'rm -rf "$work"' EXIT
python3 - "$log" << 'PY'
import json, sys
text = open(sys.argv[1]).read()
decoder, pos, ran = json.JSONDecoder(), 0, []
while pos < len(text):
    while pos < len(text) and text[pos].isspace():
        pos += 1
    if pos >= len(text):
        break
    entry, pos = decoder.raw_decode(text, pos)
    if entry.get("mnemonic") == "Krit" and not (entry.get("cacheHit") or entry.get("remoteCacheHit")):
        ran.append(entry.get("targetLabel", "?"))
if not any(label.endswith("//typed/lib:lib") for label in ran):
    sys.exit("ERROR: //typed/lib's Krit action did not re-run after its source changed: %s" % ran)
if any(label.endswith("//typed/app:app") for label in ran):
    sys.exit("ERROR: an implementation-only edit of //typed/lib re-ran //typed/app's Krit action")
print("OK: compile avoidance (re-ran: %s)" % ", ".join(ran))
PY

# Large targets' actions run in the krit_large exec group, and an
# execution platform's krit_large.<key> exec_properties reach them only.
# //:krit_large_test_platform adds krit-e2e-large=1 that way; the property
# is part of the cache key, so these actions execute (in the group, with
# the large resource estimate) instead of hitting the disk cache.
echo "== krit_large exec group: a platform's krit_large.* properties reach large Krit actions only"
large=(--extra_execution_platforms=//:krit_large_test_platform --@krit//:worker=false)
build "${extra[@]}" "${large[@]}" --@krit//:large_target_sources=1 --execution_log_json_file="$log" //typed/lib > /dev/null 2>&1 ||
  build "${extra[@]}" "${large[@]}" --@krit//:large_target_sources=1 //typed/lib
cp "$log" "$work/large.json"
build "${extra[@]}" "${large[@]}" --execution_log_json_file="$log" //typed/app > /dev/null 2>&1 ||
  build "${extra[@]}" "${large[@]}" //typed/app
python3 - "$work/large.json" "$log" << 'PY'
import json, sys
def krit_spawns(path):
    text = open(path).read()
    decoder, pos, out = json.JSONDecoder(), 0, []
    while pos < len(text):
        while pos < len(text) and text[pos].isspace():
            pos += 1
        if pos >= len(text):
            break
        entry, pos = decoder.raw_decode(text, pos)
        if entry.get("mnemonic") == "Krit":
            props = {p.get("name"): p.get("value") for p in entry.get("platform", {}).get("properties", [])}
            out.append((entry.get("targetLabel", "?"), bool(entry.get("cacheHit") or entry.get("remoteCacheHit")), props))
    return out
large, small = krit_spawns(sys.argv[1]), krit_spawns(sys.argv[2])
ran = [s for s in large if not s[1]]
if not ran:
    sys.exit("ERROR: no Krit action executed in the krit_large exec group: %s" % large)
if any(s[2].get("krit-e2e-large") != "1" for s in ran):
    sys.exit("ERROR: a large Krit action ran without the platform's krit_large property: %s" % ran)
if not small or any("krit-e2e-large" in s[2] for s in small):
    sys.exit("ERROR: a Krit action below large_target_sources got the krit_large property: %s" % small)
print("OK: krit_large (%d large Krit actions ran with the property, %d other without)" % (len(ran), len(small)))
PY

# Project-scope rules: per target, krit_aspect runs only the file-scope
# rules (--@krit//:scope=file). With --@krit//:scope=all an action over
# //project/lib alone calls usedApi dead code although //project/app calls
# it; the default does not, and //project:project_report_test (above)
# checks the whole-build report has only unusedApi.
echo "== --@krit//:scope: per-target actions skip project-scope rules"
lib_report() {
  build "${extra[@]}" --@krit//:config=//project:krit.yml "$@" //project/lib > /dev/null 2>&1 || {
    build "${extra[@]}" --@krit//:config=//project:krit.yml "$@" //project/lib
    return 1
  }
  # shellcheck disable=SC2086
  cat "$($BAZEL info "${extra[@]}" bazel-bin 2> /dev/null)/project/lib/lib.krit.txt"
}
lib_report --@krit//:scope=all | grep -q "DeadCode.*'usedApi'" || {
  echo "ERROR: --@krit//:scope=all did not run DeadCode per target" >&2
  exit 1
}
if lib_report | grep -q DeadCode; then
  echo "ERROR: the default per-target action ran the project-scope DeadCode rule" >&2
  exit 1
fi
echo "OK: scope"

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
# Capture first: a failed build must fail here, not pass as "no finding".
edited="$(report)"
if printf '%s\n' "$edited" | grep -q UnnecessarySafeCall; then
  echo "ERROR: the worker reported a finding the edited source no longer has" >&2
  exit 1
fi
# The edited source is new to every cache, so this Krit action executed.
cp "$log" "$work/edited-exec.json"
restore_consumer
restored="$(report)"
printf '%s\n' "$restored" | grep -q "typed/app/Consumer.kt:5:57: .*UnnecessarySafeCall" || {
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

# One worker JVM serves every target (#1666): its reports must match one
# JVM per action on targets that share file and declaration names.
echo "== persistent worker: one JVM for every target, the same reports as one-shot"
BAZEL="$BAZEL" bash "$repo_root/scripts/bazel-worker-parity.sh" ${extra[@]+"${extra[@]}"}

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
# Outside Bazel, Settings.kt is gated: :settings shares kotlin_settings/ with
# :annotation, so no settings-group session applies its kotlinc_opts (#1775).
# Its line-32 finding is then Go's, which fir.requireAuthoritativeRules
# withholds by default (#1675); keep it until #1775 makes the file compile.
scan_config="$work/export-model-krit.yml"
cat krit.yml > "$scan_config"
printf 'fir:\n  requireAuthoritativeRules: []\n' >> "$scan_config"
rc=0
scan="$(env -u KRIT_BAZEL_LOCAL_DIST HOME="$scan_home" KRIT_FIR_JAR="$DIST/krit-fir.jar" \
  KRIT_NO_JAR_DOWNLOAD=1 KRIT_NO_DAEMON_AUTOSTART=1 \
  "$DIST/krit" --no-daemon --no-cache --no-fir-daemon --fail-on-degraded -f plain --base-path . \
  --config "$scan_config" typed/app/Consumer.kt kotlin_settings/Settings.kt 2>&1)" || rc=$?
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
