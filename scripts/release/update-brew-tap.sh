#!/usr/bin/env bash
# Render and push the Homebrew cask for the current release tag to the
# kaeawc/homebrew-tap repository.
#
# Required env:
#   GH_TOKEN  Token with Contents: write on kaeawc/homebrew-tap (a GitHub
#             App installation token or a PAT); not needed with RENDER_ONLY
#   TAG       Release tag (e.g. v0.1.0)
#   REPO      Source repo, owner/name (e.g. kaeawc/krit)
# Optional env:
#   RENDER_ONLY  When 1, print the cask to stdout and exit without cloning
#                or pushing (used by internal/devtools/releaseassets tests)
#
# Reads SHA256s from dist/checksums.txt produced by the funnel job.

set -euo pipefail

: "${TAG:?TAG is required}"
: "${REPO:?REPO is required}"
RENDER_ONLY="${RENDER_ONLY:-0}"
if [ "$RENDER_ONLY" != "1" ]; then
  : "${GH_TOKEN:?GH_TOKEN is required}"
fi

VERSION="${TAG#v}"

# Pull SHA256 for a given archive from dist/checksums.txt.
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

# Read shas up front, before cd-ing into the tap clone (the script
# uses a relative dist/ path). Only the archives we currently publish.
DARWIN_ARM64=$(sha_for "krit_${VERSION}_darwin_arm64.tar.gz")
DARWIN_AMD64=$(sha_for "krit_${VERSION}_darwin_amd64.tar.gz")
LINUX_ARM64=$(sha_for "krit_${VERSION}_linux_arm64.tar.gz")
LINUX_AMD64=$(sha_for "krit_${VERSION}_linux_amd64.tar.gz")

URL_BASE="https://github.com/${REPO}/releases/download/${TAG}"

# Stanza order follows `brew style` (the tap's audit job runs it on every
# push): sha256 before url inside each platform block, and no blank line
# between on_macos and on_linux.
render_cask() {
  cat <<EOF
cask "krit" do
  version "${VERSION}"

  on_macos do
    on_arm do
      sha256 "${DARWIN_ARM64}"
      url "${URL_BASE}/krit_#{version}_darwin_arm64.tar.gz"
    end
    on_intel do
      sha256 "${DARWIN_AMD64}"
      url "${URL_BASE}/krit_#{version}_darwin_amd64.tar.gz"
    end
  end
  on_linux do
    on_arm do
      sha256 "${LINUX_ARM64}"
      url "${URL_BASE}/krit_#{version}_linux_arm64.tar.gz"
    end
    on_intel do
      sha256 "${LINUX_AMD64}"
      url "${URL_BASE}/krit_#{version}_linux_amd64.tar.gz"
    end
  end

  name "krit"
  desc "Fast Kotlin static analysis powered by tree-sitter"
  homepage "https://github.com/${REPO}"

  binary "krit"
  binary "krit-lsp"
  binary "krit-mcp"
end
EOF
}

if [ "$RENDER_ONLY" = "1" ]; then
  render_cask
  exit 0
fi

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
cd "$WORKDIR"

# Clone with token-auth so we can push back.
if ! git clone "https://x-access-token:${GH_TOKEN}@github.com/kaeawc/homebrew-tap.git" .; then
  echo "::error::Could not clone kaeawc/homebrew-tap with the release token. Install the release GitHub App (GH_APP_ID) on kaeawc/homebrew-tap, or rotate the HOMEBREW_TAP_TOKEN secret (Contents: write)."
  exit 1
fi
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

mkdir -p Casks
render_cask > Casks/krit.rb

git add Casks/krit.rb
if git diff --cached --quiet; then
  echo "no changes to brew cask"
  exit 0
fi
git commit -m "krit ${TAG}"
git push origin HEAD
