#!/usr/bin/env bash
# Render and push the Scoop manifest for the current release tag to the
# kaeawc/scoop-krit bucket repository.
#
# Required env:
#   GH_TOKEN  PAT with Contents:Write on kaeawc/scoop-krit
#   TAG       Release tag (e.g. v0.1.0)
#   REPO      Source repo, owner/name (e.g. kaeawc/krit)

set -euo pipefail

: "${GH_TOKEN:?GH_TOKEN is required}"
: "${TAG:?TAG is required}"
: "${REPO:?REPO is required}"

VERSION="${TAG#v}"

sha_for() {
  local name="$1"
  local sha
  sha=$(grep "  ${name}\$" dist/checksums.txt | awk '{print $1}' || true)
  if [ -z "$sha" ]; then
    echo "ERROR: no checksum for $name in dist/checksums.txt" >&2
    exit 1
  fi
  printf '%s' "$sha"
}

WIN_AMD64=$(sha_for "krit_${VERSION}_windows_amd64.zip")
URL_BASE="https://github.com/${REPO}/releases/download/${TAG}"

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
cd "$WORKDIR"

git clone "https://x-access-token:${GH_TOKEN}@github.com/kaeawc/scoop-krit.git" .
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

mkdir -p bucket
cat > bucket/krit.json <<EOF
{
  "version": "${VERSION}",
  "description": "Fast Kotlin static analysis powered by tree-sitter",
  "homepage": "https://github.com/${REPO}",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "${URL_BASE}/krit_${VERSION}_windows_amd64.zip",
      "hash": "${WIN_AMD64}"
    }
  },
  "bin": [
    "krit.exe",
    "krit-lsp.exe",
    "krit-mcp.exe"
  ],
  "checkver": "github",
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/${REPO}/releases/download/v\$version/krit_\$version_windows_amd64.zip"
      }
    }
  }
}
EOF

git add bucket/krit.json
if git diff --cached --quiet; then
  echo "no changes to scoop manifest"
  exit 0
fi
git commit -m "krit ${TAG}"
git push origin HEAD
