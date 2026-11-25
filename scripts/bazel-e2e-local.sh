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
