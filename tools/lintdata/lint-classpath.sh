#!/bin/bash
# Prints the runtime classpath of the Android Lint command line tool (com.android.tools.lint:lint:$LINT_VERSION).
# Reads LINT_CLASSPATH_FILE when set; otherwise resolves the artifact with the Gradle wrapper in tools/krit-fir,
# which works offline once the artifacts are in the Gradle cache.
set -euo pipefail
if [ -n "${LINT_CLASSPATH_FILE:-}" ]; then cat "$LINT_CLASSPATH_FILE"; exit 0; fi
here="$(cd "$(dirname "$0")" && pwd)"
ver="${LINT_VERSION:-32.4.1}"
tmp="$(mktemp -d)"
cat > "$tmp/settings.gradle.kts" <<'S'
rootProject.name = "lintcp"
S
cat > "$tmp/build.gradle.kts" <<G
plugins { java }
repositories { google(); mavenCentral() }
dependencies { implementation("com.android.tools.lint:lint:$ver") }
tasks.register("printClasspath") { doLast { println("CLASSPATH=" + configurations.runtimeClasspath.get().asPath) } }
G
"$here/../krit-fir/gradlew" -q -p "$tmp" printClasspath | sed -n 's/^CLASSPATH=//p'
rm -rf "$tmp"
