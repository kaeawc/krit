#!/bin/bash
# Regenerates internal/rules/lint_tables_gen.go from Android Lint's manifest detectors.
# usage: tools/lintdata/regen-tables.sh   (LINT_VERSION picks the version, default 32.4.1)
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
ver="${LINT_VERSION:-32.4.1}"
cache="$HOME/.gradle/caches/modules-2/files-2.1"
jar() { find "$cache/$1/$2/$ver" -name "$2-$ver.jar" | head -1; }
cp="$(jar com.android.tools.lint lint-checks):$(jar com.android.tools.lint lint-api):$(jar com.android.tools lint-typedef-remover 2>/dev/null || true)"
for dep in com.google.guava/guava org.jetbrains.kotlin/kotlin-stdlib; do
  cp="$cp:$(find "$cache/$dep" -name '*.jar' ! -name '*sources*' | head -1)"
done
cp="$cp:$(jar com.android.tools common):$(jar com.android.tools sdk-common):$(jar com.android.tools sdklib)"
work="$(mktemp -d)"
javac -cp "$cp" -d "$work" "$here/GenManifestTables.java"
java -cp "$work:$cp" GenManifestTables > "$repo/internal/rules/lint_tables_gen.go"
gofmt -w "$repo/internal/rules/lint_tables_gen.go"
rm -rf "$work"
