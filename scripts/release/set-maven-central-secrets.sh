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

# Reads one key with java.util.Properties semantics: `=`, `:` or whitespace
# separators with optional surrounding blanks, CRLF or LF line endings,
# `#`/`!` comments, backslash line continuations and escapes (so a
# signingInMemoryKey written with literal "\n" escapes comes back with real
# newlines, as Gradle reads it). Prints the value without a trailing newline.
prop() {
  python3 - "$props" "$1" <<'PY'
import re, sys

path, wanted = sys.argv[1], sys.argv[2]
with open(path, encoding="latin-1", newline="") as fh:
    raw = re.split(r"\r\n|\r|\n", fh.read())

def logical_lines(lines):
    buf = None
    for line in lines:
        stripped = line.lstrip(" \t\f")
        if buf is None:
            if not stripped or stripped[0] in "#!":
                continue
            buf = stripped
        else:
            buf += stripped
        trailing = len(buf) - len(buf.rstrip("\\"))
        if trailing % 2 == 1:
            buf = buf[:-1]
            continue
        yield buf
        buf = None
    if buf is not None:
        yield buf

def unescape(text):
    out, i = [], 0
    while i < len(text):
        c = text[i]
        if c != "\\" or i + 1 == len(text):
            out.append(c)
            i += 1
            continue
        n = text[i + 1]
        if n == "u" and re.fullmatch(r"[0-9a-fA-F]{4}", text[i + 2:i + 6] or ""):
            out.append(chr(int(text[i + 2:i + 6], 16)))
            i += 6
            continue
        out.append({"t": "\t", "n": "\n", "r": "\r", "f": "\f"}.get(n, n))
        i += 2
    return "".join(out)

for line in logical_lines(raw):
    i = 0
    while i < len(line):
        if line[i] == "\\":
            i += 2
            continue
        if line[i] in "=: \t\f":
            break
        i += 1
    key = unescape(line[:i])
    rest = line[i:].lstrip(" \t\f")
    if rest[:1] in ("=", ":") and (i == len(line) or line[i] in " \t\f"):
        rest = rest[1:].lstrip(" \t\f")
    elif line[i:i + 1] in ("=", ":"):
        rest = line[i + 1:].lstrip(" \t\f")
    if key == wanted:
        value = unescape(rest)
if "value" in dir():
    sys.stdout.write(value)
PY
}

# Read and check every value before the first `gh secret set`, so a bad
# file never overwrites a working secret with an empty or partial value.
username="$(prop mavenCentralUsername)"
password="$(prop mavenCentralPassword)"
signing_key="$(prop signingInMemoryKey)"
signing_password="$(prop signingInMemoryKeyPassword)"

missing=()
[ -n "$username" ] || missing+=(mavenCentralUsername)
[ -n "$password" ] || missing+=(mavenCentralPassword)
[ -n "$signing_key" ] || missing+=(signingInMemoryKey)
[ -n "$signing_password" ] || missing+=(signingInMemoryKeyPassword)
if [ "${#missing[@]}" -gt 0 ]; then
  echo "set-maven-central-secrets: $props is missing ${missing[*]}" >&2
  exit 1
fi
case "$signing_key" in
  "-----BEGIN PGP PRIVATE KEY BLOCK-----"*) ;;
  *)
    echo "set-maven-central-secrets: signingInMemoryKey is not an ASCII-armored private key" >&2
    exit 1
    ;;
esac

printf '%s\n' "$signing_key" | gh secret set MAVEN_CENTRAL_SIGNING_KEY -R "$repo"
printf '%s' "$signing_password" | gh secret set MAVEN_CENTRAL_SIGNING_PASSWORD -R "$repo"
printf '%s' "$username" | gh secret set SONATYPE_USERNAME -R "$repo"
printf '%s' "$password" | gh secret set SONATYPE_PASSWORD -R "$repo"

gh secret list -R "$repo" | grep -E '^(MAVEN_CENTRAL_SIGNING_KEY|MAVEN_CENTRAL_SIGNING_PASSWORD|SONATYPE_USERNAME|SONATYPE_PASSWORD)\b'
