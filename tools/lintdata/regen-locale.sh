#!/bin/bash
# Regenerates internal/rules/locale_data_gen.go from Android Lint's LocaleManager.
# usage: tools/lintdata/regen-locale.sh [path/to/sdk-common.jar] [path/to/common.jar]
# Default jars come from the Gradle cache (version LINT_VERSION, default 32.4.1).
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
ver="${LINT_VERSION:-32.4.1}"
cache="$HOME/.gradle/caches/modules-2/files-2.1"
sdk="${1:-$(find "$cache/com.android.tools/sdk-common/$ver" -name "sdk-common-$ver.jar" | head -1)}"
com="${2:-$(find "$cache/com.android.tools/common/$ver" -name "common-$ver.jar" | head -1)}"
guava="$(find "$cache/com.google.guava/guava" -name 'guava-3*-jre.jar' | head -1)"
work="$(mktemp -d)"
javac -cp "$sdk:$com:$guava" -d "$work" "$here/GenLocale.java"
java -cp "$work:$sdk:$com:$guava" GenLocale > "$repo/internal/rules/locale_data_gen.go"
gofmt -w "$repo/internal/rules/locale_data_gen.go"
rm -rf "$work"
