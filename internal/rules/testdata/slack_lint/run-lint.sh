#!/bin/bash
# Prints what Android Lint reports, with slack-lint-checks on the rule path, for one parity case directory:
# one `path:line:column: ID: message` line per finding whose ID is in the case's checks.txt, sorted.
#
#   run-lint.sh <case-dir> <stub-dir>... > <case-dir>/lint-0.11.1.txt
#
# Needs: LINT (the `lint` launcher from Android cmdline-tools 16111833, Lint 9.4.0-rc01), SLACK_LINT_JAR
# (com.slack.lint:slack-lint-checks:0.11.1), KOTLIN_STDLIB_JAR (kotlin-stdlib 2.2.21) and a JDK 21 in JAVA_HOME.
# See README.md.
set -euo pipefail
case_dir=$(cd "$1" && pwd)
shift
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/src" "$tmp/libs"
cp -r "$case_dir/src/main/java/." "$tmp/src/"
for stubs in "$@"; do cp -r "$stubs/." "$tmp/src/"; done
cp "$KOTLIN_STDLIB_JAR" "$tmp/libs/"
# Directory mode needs a manifest whose package has a dot.
echo '<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.fixture"><uses-sdk android:minSdkVersion="21"/></manifest>' > "$tmp/AndroidManifest.xml"
"$LINT" --quiet --lint-rule-jars "$SLACK_LINT_JAR" --disable LintError,MissingVersion --xml "$tmp/out.xml" "$tmp" >/dev/null 2>&1 || true
python3 -I "$here/lint-xml.py" "$tmp/out.xml" "$tmp" "$(tr -d '[:space:]' < "$case_dir/checks.txt")"
