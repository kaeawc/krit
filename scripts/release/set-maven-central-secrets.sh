#!/usr/bin/env bash
# Copies Maven Central publishing credentials from a Gradle properties file
# into the GitHub Actions secrets publish-maven-central.yml reads. Values are
# piped straight to `gh secret set` and never printed.
#
# Reads the keys the vanniktech maven-publish plugin uses, so an existing
# Central publishing setup works unchanged:
#   mavenCentralUsername / mavenCentralPassword    -> SONATYPE_USERNAME / SONATYPE_PASSWORD
#   signingInMemoryKey / signingInMemoryKeyPassword -> MAVEN_CENTRAL_SIGNING_KEY / MAVEN_CENTRAL_SIGNING_PASSWORD
# signingInMemoryKey may hold literal "\n" escapes; they become real newlines.
#
# Usage: scripts/release/set-maven-central-secrets.sh [properties-file] [owner/repo]
set -euo pipefail

props="${1:-${GRADLE_USER_HOME:-$HOME/.gradle}/gradle.properties}"
repo="${2:-kaeawc/krit}"

if [ ! -f "$props" ]; then
  echo "set-maven-central-secrets: $props not found" >&2
  exit 1
fi

prop() {
  grep -E "^$1=" "$props" | head -1 | cut -d= -f2-
}

missing=()
for key in mavenCentralUsername mavenCentralPassword signingInMemoryKey signingInMemoryKeyPassword; do
  [ -n "$(prop "$key")" ] || missing+=("$key")
done
if [ "${#missing[@]}" -gt 0 ]; then
  echo "set-maven-central-secrets: $props is missing ${missing[*]}" >&2
  exit 1
fi

prop signingInMemoryKey | python3 -c '
import sys
key = sys.stdin.read().strip().replace("\\n", "\n")
if not key.startswith("-----BEGIN PGP PRIVATE KEY BLOCK-----"):
    sys.exit("set-maven-central-secrets: signingInMemoryKey is not an ASCII-armored private key")
sys.stdout.write(key + "\n")
' | gh secret set MAVEN_CENTRAL_SIGNING_KEY -R "$repo"
printf '%s' "$(prop signingInMemoryKeyPassword)" | gh secret set MAVEN_CENTRAL_SIGNING_PASSWORD -R "$repo"
printf '%s' "$(prop mavenCentralUsername)" | gh secret set SONATYPE_USERNAME -R "$repo"
printf '%s' "$(prop mavenCentralPassword)" | gh secret set SONATYPE_PASSWORD -R "$repo"

gh secret list -R "$repo" | grep -E '^(MAVEN_CENTRAL_SIGNING_KEY|MAVEN_CENTRAL_SIGNING_PASSWORD|SONATYPE_USERNAME|SONATYPE_PASSWORD)\b'
