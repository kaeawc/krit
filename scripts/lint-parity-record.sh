#!/usr/bin/env bash
# Records what Android Lint reports for a project fixture, for krit's Lint parity tests.
#
#   scripts/lint-parity-record.sh tests/fixtures/android-lint/<RuleID>/<case> [IssueId,IssueId...]
#
# A case directory is a Gradle module (build.gradle.kts or build.gradle plus src/main/...). The script copies it as the :app module
# of a throwaway project that applies AGP 9.4.1 (which bundles Lint 32.4.1), runs `lint` with only the given
# issues (default: the <RuleID> directory name), and writes the findings to <case>/lint-32.4.1.txt as sorted
# `path:line:column: ID: message` lines with paths relative to the case. cmd/krit's TestAndroidLintParityFixtures
# replays krit on the same directory and compares. An Unreleased CHANGELOG entry that says its rules match Lint on
# "N recorded project fixtures" counts the <RuleID>/<case>/lint-32.4.1.txt cases of the rules it names in backticks;
# cmd/krit's TestChangelogLintParityFixtureCounts checks it. Dependencies in the fixture's build.gradle.kts are resolved, so
# checks that need a library on the classpath see it. Compile tasks are skipped: Lint reads sources through UAST and
# fixtures need not compile.
#
# A case whose resources AGP 9.4.1 cannot compile (an unescaped apostrophe, for one) never reaches lintDebug. Such a
# case holds lint-standalone.txt saying why, and the script runs Lint 32.4.1's own CLI (com.android.tools.lint.Main,
# resolved from Maven) on the case's src/main laid out as a non-Gradle project instead. No dependencies are on the
# classpath there, so use it only for resource checks.
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

# print_findings <lint-xml> <root> <prefix-map>: sorted `path:line:column: ID: message` lines. With prefix-map
# "standalone", paths of the flattened project map back to the module layout (res/ -> src/main/res/, src/ ->
# src/main/java/, AndroidManifest.xml -> src/main/AndroidManifest.xml).
print_findings() {
	python3 -I - "$1" "$2" "$3" <<'PY'
import os, sys, xml.etree.ElementTree as ET
report, root, mode = sys.argv[1:4]
out = []
for issue in ET.parse(report).getroot().iter("issue"):
    loc = issue.find("location")
    if loc is None:
        continue
    path = loc.get("file")
    path = os.path.relpath(path, root) if os.path.isabs(path) else path
    if mode == "standalone":
        path = "src/main/java/" + path[len("src/"):] if path.startswith("src/") else "src/main/" + path
    out.append(f"{path}:{loc.get('line', '0')}:{loc.get('column', '0')}: {issue.get('id')}: {issue.get('message')}")
for line in sorted(out):
    print(line)
PY
}

if [ -f "$case_dir/lint-standalone.txt" ]; then
	cli="$work/standalone-cli"
	project="$work/standalone-project"
	rm -rf "$project"
	mkdir -p "$cli" "$project"
	printf '%s\n' 'dependencyResolutionManagement { repositories { google(); mavenCentral() } }' \
		'rootProject.name = "lint-standalone"' > "$cli/settings.gradle.kts"
	printf '%s\n' 'plugins { java }' 'dependencies { runtimeOnly("com.android.tools.lint:lint:32.4.1") }' \
		'tasks.register<JavaExec>("lint") {' \
		'    classpath = sourceSets["main"].runtimeClasspath' \
		'    mainClass.set("com.android.tools.lint.Main")' \
		'    args = (project.property("lintArgs") as String).split("|")' \
		'    isIgnoreExitValue = true' '}' > "$cli/build.gradle.kts"
	main="$case_dir/src/main"
	namespace=$(sed -n 's/^ *namespace *= *"\([^"]*\)".*/\1/p' "$case_dir/build.gradle.kts" | head -1)
	if [ -f "$main/AndroidManifest.xml" ]; then
		sed "s|<manifest |<manifest package=\"$namespace\" |" "$main/AndroidManifest.xml" > "$project/AndroidManifest.xml"
	fi
	if [ -d "$main/res" ]; then cp -r "$main/res" "$project/res"; fi
	if [ -d "$main/java" ]; then cp -r "$main/java" "$project/src"; fi
	if ! (cd "$cli" && "$gradle" -q --console=plain lint \
		"-PlintArgs=--check|$checks|--sdk-home|$ANDROID_HOME|--xml|$project/lint-out.xml|--disable|LintError|$project" \
		>"$work/gradle.log" 2>&1); then
		tail -30 "$work/gradle.log" >&2
		exit 1
	fi
	print_findings "$project/lint-out.xml" "$project" standalone > "$case_dir/lint-32.4.1.txt"
	cat "$case_dir/lint-32.4.1.txt"
	exit 0
fi

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
rm -f "$work/app"/lint-*.txt "$work/app"/divergences.txt "$work/app"/severity.txt "$work/app"/lint-standalone.txt
if [ -f "$work/app/build.gradle" ]; then
	cat >> "$work/app/build.gradle" <<EOF

android {
    lint {
        checkOnly '${checks//,/\', \'}'
        abortOnError false
        xmlReport true
        xmlOutput file("lint-out.xml")
        checkReleaseBuilds false
    }
}
EOF
else
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
fi
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

# Library modules order some tasks after compilation, so retry with compilation when skipping it fails.
# `lint` checks the default variant (lintDebug without flavors); with flavors the compile task names differ, so the
# retry without -x covers them too.
if ! (cd "$work" && "$gradle" -q --console=plain :app:lint -x compileDebugKotlin -x compileDebugJavaWithJavac \
	>"$work/gradle.log" 2>&1) && ! (cd "$work" && "$gradle" -q --console=plain :app:lint >"$work/gradle.log" 2>&1); then
	tail -30 "$work/gradle.log" >&2
	exit 1
fi
print_findings "$work/app/lint-out.xml" "$work/app" module > "$case_dir/lint-32.4.1.txt"
cat "$case_dir/lint-32.4.1.txt"
