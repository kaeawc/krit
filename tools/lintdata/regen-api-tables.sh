#!/bin/bash
# Regenerates internal/rules/lint_api_tables_gen.go from Android Lint's API database.
# usage: tools/lintdata/regen-api-tables.sh [path/to/api-versions.xml]
# Without an argument it uses the newest SDK platform under $ANDROID_HOME (or ~/Library/Android/sdk).
# The Lint classpath comes from lint-classpath.sh (set LINT_CLASSPATH_FILE to reuse a saved one).
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
sdk="${ANDROID_HOME:-$HOME/Library/Android/sdk}"
xml="${1:-$(ls -d "$sdk"/platforms/android-*/data/api-versions.xml | sort -V | tail -1)}"
cp="$("$here/lint-classpath.sh")"
work="$(mktemp -d)"
javac -cp "$cp" -d "$work" "$here/GenApiTables.java"
java -cp "$work:$cp" GenApiTables "$xml" > "$repo/internal/rules/lint_api_tables_gen.go"
gofmt -w "$repo/internal/rules/lint_api_tables_gen.go"
rm -rf "$work"
