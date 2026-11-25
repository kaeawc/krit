#!/usr/bin/env bash
# End-to-end check of the krit Bazel module (bazel/) against the analyzer
# built from this checkout: krit from cmd/krit and the krit-fir jar from
# tools/krit-fir, handed to the module through KRIT_BAZEL_LOCAL_DIST instead
# of a release download. Runs `bazel test //...` in bazel/e2e twice with a
# disk cache, and `bazel clean` in between; the second run must be a full
# cache hit (every test "(cached)"), which checks that the krit actions and
# tests are deterministic and carry no machine-specific inputs, so a remote
# cache can share them.
#
# Usage: scripts/bazel-e2e-local.sh [extra bazel test args...]
#
# Optional env:
#   KRIT_FIR_JAR  krit-fir jar to use (default: `make fir-jar`, which builds
#                 it once per source hash)
#   BAZEL         Bazel launcher (default bazel; bazelisk honours
#                 bazel/e2e/.bazelversion and USE_BAZEL_VERSION)
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
