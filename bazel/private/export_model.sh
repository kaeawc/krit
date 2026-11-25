#!/usr/bin/env bash
# bazel run @krit//:export_model -- [bazel build args] [targets]
#
# Builds the krit_model output group of krit_model_aspect for the targets
# (default //...) and installs one model file per target into
# .krit/bazel-model/ in the workspace, replacing the previous export. krit,
# krit-lsp and krit-mcp discover that directory like .krit/gradle-model/.
# Paths in the files are absolute: sources in the workspace, jars and
# generated files in Bazel's execution root (bazel-out, external). Re-run
# it after changing BUILD files or deps; krit warns about entries that have
# disappeared (bazel clean).
#
# Env: BAZEL (default bazel) is the Bazel launcher; KRIT_MODEL_ASPECT
# overrides the aspect label (default @krit//:defs.bzl%krit_model_aspect).
set -euo pipefail

workspace="${BUILD_WORKSPACE_DIRECTORY:?run this with bazel run @krit//:export_model}"
cd "$workspace"
bazel_cmd="${BAZEL:-bazel}"
aspect="${KRIT_MODEL_ASPECT:-@krit//:defs.bzl%krit_model_aspect}"

args=("$@")
has_target=0
for a in "${args[@]}"; do
  case "$a" in
    -*) ;;
    *) has_target=1 ;;
  esac
done
if [ "$has_target" = 0 ]; then
  args+=("//...")
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
bep="$work/bep.json"

# shellcheck disable=SC2086
$bazel_cmd build \
  --aspects="$aspect" \
  --output_groups=krit_model \
  --build_event_json_file="$bep" \
  --remote_download_outputs=toplevel \
  "${args[@]}"
# shellcheck disable=SC2086
execroot="$($bazel_cmd info execution_root)"

# Escapes a replacement for sed's s|...|...|.
sed_escape() { printf '%s' "$1" | sed -e 's/[|&\\]/\\&/g'; }
workspace_re="$(sed_escape "$workspace")"
execroot_re="$(sed_escape "$execroot")"

out="$workspace/.krit/bazel-model"
rm -rf "$out"
mkdir -p "$out"
count=0
# The BEP names every output of the build by file:// URI.
while IFS= read -r uri; do
  file="${uri#file://}"
  [ -f "$file" ] || continue
  rel="${file#*/bin/}"
  name="$(printf '%s' "$rel" | tr '/' '_')"
  sed -e "s|@@WORKSPACE@@|${workspace_re}|g" \
    -e "s|@@EXECROOT@@|${execroot_re}|g" "$file" > "$out/$name"
  count=$((count + 1))
done < <(grep -o '"uri":"file://[^"]*\.krit-model\.json"' "$bep" | sed -e 's/^"uri":"//' -e 's/"$//' | sort -u)

if [ "$count" = 0 ]; then
  echo "export_model: no Kotlin or Java targets in ${args[*]}" >&2
  exit 1
fi
echo "export_model: wrote $count target models to $out"
