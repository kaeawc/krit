#!/usr/bin/env bash
set -euo pipefail

tool="${1:-}"
case "$tool" in
  krit-fir|krit-types) ;;
  *) echo "usage: scripts/build-dev-jar.sh krit-fir|krit-types" >&2; exit 2 ;;
esac

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"
source_hash="$(go run ./cmd/krit-dev-jar-hash "$tool")"
target_dir="$HOME/.krit/jars/dev/$source_hash"
target="$target_dir/$tool.jar"
if [[ -f "$target" ]]; then
  echo "$target"
  exit 0
fi

(cd "tools/$tool" && ./gradlew shadowJar)
mkdir -p "$target_dir"
# BSD mktemp only substitutes trailing X placeholders.
temp_jar="$(mktemp "$target_dir/.$tool.jar.XXXXXXXX")"
trap 'rm -f "$temp_jar"' EXIT
cp "tools/$tool/build/libs/$tool.jar" "$temp_jar"
mv "$temp_jar" "$target"
echo "$target"
