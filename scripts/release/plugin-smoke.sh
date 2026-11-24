#!/usr/bin/env bash
# Pre-publish smoke test for the krit Gradle plugins.
#
# Resolves dev.jasonpearson.krit.settings and dev.jasonpearson.krit from the
# staged (not yet uploaded) Maven layouts in a throwaway Gradle project, so a
# broken POM, module file, or plugin marker fails the release before either
# plugin is uploaded to Sonatype Central. Only Maven Central / the Gradle Plugin Portal are
# contacted (for the Kotlin stdlib and other transitive dependencies); the
# krit group itself is resolved exclusively from the staging directories.
#
# Usage: scripts/release/plugin-smoke.sh VERSION STAGING_DIR [STAGING_DIR...]
#   VERSION      version the plugins were staged with (no leading v)
#   STAGING_DIR  a build/staging-deploy directory (or any local Maven repo)
#                holding the published layout; pass one per plugin
set -euo pipefail

if [ "$#" -lt 2 ]; then
  printf 'usage: %s VERSION STAGING_DIR [STAGING_DIR...]\n' "$0" >&2
  exit 2
fi

version="$1"
shift

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
wrapper="${repo_root}/krit-settings-plugin/gradlew"

staging_repos=""
for dir in "$@"; do
  if [ ! -d "$dir" ]; then
    printf 'plugin-smoke: staging directory not found: %s\n' "$dir" >&2
    exit 2
  fi
  abs="$(cd "$dir" && pwd)"
  staging_repos="${staging_repos}        maven {
            url = uri(\"${abs}\")
            content { includeGroupAndSubgroups(\"dev.jasonpearson.krit\") }
        }
"
done

tmp="$(mktemp -d "${TMPDIR:-/tmp}/krit-plugin-smoke.XXXXXX")"
cleanup() {
  if [ -n "${tmp:-}" ] && [ -d "$tmp" ]; then
    rm -rf "$tmp"
  fi
}
trap cleanup EXIT

plugin_management="pluginManagement {
    repositories {
${staging_repos}        // The krit group must come from staging, never from a published release.
        mavenCentral { content { excludeGroupAndSubgroups(\"dev.jasonpearson.krit\") } }
        gradlePluginPortal { content { excludeGroupAndSubgroups(\"dev.jasonpearson.krit\") } }
    }
}"

# Runs :app:tasks in the project at $1 and requires a kritCheck task.
run_project() {
  local dir="$1" label="$2" out="$1/tasks.out"
  printf 'plugin-smoke: %s (%s from staging)\n' "$label" "$version"
  if ! "$wrapper" -p "$dir" --no-daemon --console=plain :app:tasks --all >"$out" 2>&1; then
    tail -60 "$out" >&2
    printf 'plugin-smoke: FAILED: %s\n' "$label" >&2
    exit 1
  fi
  if ! grep -q 'kritCheck' "$out"; then
    tail -60 "$out" >&2
    printf 'plugin-smoke: FAILED: %s resolved but :app has no kritCheck task\n' "$label" >&2
    exit 1
  fi
}

# 1. The settings plugin: resolves the marker, the plugin jar, and its
#    krit-gradle-plugin dependency, then auto-applies krit to the Kotlin project.
mkdir -p "${tmp}/settings-plugin/app"
cat > "${tmp}/settings-plugin/settings.gradle.kts" <<EOF
${plugin_management}

plugins {
    id("dev.jasonpearson.krit.settings") version "${version}"
}

rootProject.name = "krit-settings-smoke"
include(":app")
EOF
# The auto-apply hook keys on Kotlin / Android plugin ids (not plain `java`);
# the Kotlin Gradle plugin resolves from the Plugin Portal.
cat > "${tmp}/settings-plugin/app/build.gradle.kts" <<EOF
plugins {
    kotlin("jvm") version "2.3.21"
}
EOF
run_project "${tmp}/settings-plugin" "dev.jasonpearson.krit.settings"

# 2. The project plugin on its own, through its own marker.
mkdir -p "${tmp}/project-plugin/app"
cat > "${tmp}/project-plugin/settings.gradle.kts" <<EOF
${plugin_management}

rootProject.name = "krit-project-smoke"
include(":app")
EOF
cat > "${tmp}/project-plugin/app/build.gradle.kts" <<EOF
plugins {
    java
    id("dev.jasonpearson.krit") version "${version}"
}
EOF
run_project "${tmp}/project-plugin" "dev.jasonpearson.krit"

printf 'plugin-smoke: OK\n'
