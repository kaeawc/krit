#!/usr/bin/env bash
# Render and push the Scoop manifest for the current release tag to the
# kaeawc/scoop-krit bucket repository.
#
# Required env:
#   GH_TOKEN  Token with Contents: write on kaeawc/scoop-krit (a GitHub
#             App installation token or a PAT); not needed with RENDER_ONLY
#   TAG       Release tag (e.g. v0.1.0)
#   REPO      Source repo, owner/name (e.g. kaeawc/krit)
# Optional env:
#   RENDER_ONLY  When 1, print the manifest to stdout and exit without
#                cloning or pushing (used by internal/devtools/releaseassets
#                tests)

set -euo pipefail

: "${TAG:?TAG is required}"
: "${REPO:?REPO is required}"
RENDER_ONLY="${RENDER_ONLY:-0}"
if [ "$RENDER_ONLY" != "1" ]; then
  : "${GH_TOKEN:?GH_TOKEN is required}"
fi

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

render_manifest() {
  cat <<EOF
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
}

if [ "$RENDER_ONLY" = "1" ]; then
  render_manifest
  exit 0
fi

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
cd "$WORKDIR"

if ! git clone "https://x-access-token:${GH_TOKEN}@github.com/kaeawc/scoop-krit.git" .; then
  echo "::error::Could not clone kaeawc/scoop-krit with the release token. Install the release GitHub App (GH_APP_ID) on kaeawc/scoop-krit, or rotate the SCOOP_BUCKET_TOKEN secret (Contents: write)."
  exit 1
fi
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

mkdir -p bucket
render_manifest > bucket/krit.json

git add bucket/krit.json
if git diff --cached --quiet; then
  echo "no changes to scoop manifest"
  exit 0
fi
git commit -m "krit ${TAG}"
git push origin HEAD
