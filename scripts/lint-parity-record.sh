#!/usr/bin/env bash
# Records what Android Lint reports for a project fixture, for krit's Lint parity tests.
#
#   scripts/lint-parity-record.sh tests/fixtures/android-lint/<RuleID>/<case> [IssueId,IssueId...]
#
# A case directory is a Gradle module (build.gradle.kts plus src/main/...). The script copies it as the :app module
# of a throwaway project that applies AGP 9.4.1 (which bundles Lint 32.4.1), runs `lintDebug` with only the given
# issues (default: the <RuleID> directory name), and writes the findings to <case>/lint-32.4.1.txt as sorted
# `path:line:column: ID: message` lines with paths relative to the case. cmd/krit's TestAndroidLintParityFixtures
# replays krit on the same directory and compares. Dependencies in the fixture's build.gradle.kts are resolved, so
# checks that need a library on the classpath see it. Compile tasks are skipped: Lint reads sources through UAST and
# fixtures need not compile.
#
# Needs JAVA_HOME (JDK 17+), ANDROID_HOME (an SDK with platforms;android-36), and GRADLE (a Gradle 9 launcher;
# default `gradle`). KRIT_LINT_PARITY_WORK sets the scratch project directory (default ~/.cache/krit-lint-parity).
set -euo pipefail

case_dir=${1:?usage: $0 <case-dir> [issue-ids]}
case_dir=$(cd "$case_dir" && pwd)
checks=${2:-$(basename "$(dirname "$case_dir")")}
: "${JAVA_HOME:?set JAVA_HOME to a JDK 17+}"
: "${ANDROID_HOME:?set ANDROID_HOME to an Android SDK}"
gradle=${GRADLE:-gradle}
work=${KRIT_LINT_PARITY_WORK:-$HOME/.cache/krit-lint-parity}

mkdir -p "$work"
rm -rf "$work/app"
cat > "$work/settings.gradle.kts" <<'EOF'
pluginManagement { repositories { google(); mavenCentral(); gradlePluginPortal() } }
dependencyResolutionManagement { repositories { google(); mavenCentral() } }
include(":app")
EOF
cat > "$work/build.gradle.kts" <<'EOF'
plugins {
    id("com.android.application") version "9.4.1" apply false
    id("com.android.library") version "9.4.1" apply false
}
EOF
printf 'android.useAndroidX=true\norg.gradle.jvmargs=-Xmx3g\n' > "$work/gradle.properties"
printf 'sdk.dir=%s\n' "$ANDROID_HOME" > "$work/local.properties"

cp -r "$case_dir" "$work/app"
rm -f "$work/app"/lint-*.txt "$work/app"/divergences.txt
cat >> "$work/app/build.gradle.kts" <<EOF

android {
    lint {
        checkOnly += setOf("${checks//,/\", \"}")
        abortOnError = false
        xmlReport = true
        xmlOutput = file("lint-out.xml")
        checkReleaseBuilds = false
    }
}
EOF
# AGP 9 rejects package= and <uses-sdk> versions in a manifest; drop them without moving any line.
find "$work/app/src" -name AndroidManifest.xml | while read -r manifest; do
	python3 -I - "$manifest" <<'PY'
import re, sys
path = sys.argv[1]
text = open(path).read()
text = re.sub(r' package="[^"]*"', '', text)
text = re.sub(r'<uses-sdk[^>]*/>', lambda m: '\n' * m.group(0).count('\n'), text)
open(path, 'w').write(text)
PY
done

if ! (cd "$work" && "$gradle" -q --console=plain :app:lintDebug -x compileDebugKotlin -x compileDebugJavaWithJavac \
	>"$work/gradle.log" 2>&1); then
	tail -30 "$work/gradle.log" >&2
	exit 1
fi
python3 -I - "$work/app" > "$case_dir/lint-32.4.1.txt" <<'PY'
import os, sys, xml.etree.ElementTree as ET
app = sys.argv[1]
out = []
for issue in ET.parse(os.path.join(app, "lint-out.xml")).getroot().iter("issue"):
    loc = issue.find("location")
    if loc is None:
        continue
    path = os.path.relpath(loc.get("file"), app)
    out.append(f"{path}:{loc.get('line', '0')}:{loc.get('column', '0')}: {issue.get('id')}: {issue.get('message')}")
for line in sorted(out):
    print(line)
PY
cat "$case_dir/lint-32.4.1.txt"
