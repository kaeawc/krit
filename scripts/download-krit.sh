#!/usr/bin/env bash
set -euo pipefail

# Downloads the krit binary for the current platform.
# Expected environment variables (set by the GitHub Action):
#   KRIT_VERSION_BARE  - version without leading v (e.g. 0.2.0)
#   KRIT_OS            - operating system (linux, darwin)
#   KRIT_ARCH          - architecture (amd64, arm64)
#   KRIT_TAG           - full tag (e.g. v0.2.0)
#   KRIT_INSTALL_DIR   - directory to install into

VERSION="${KRIT_VERSION_BARE:?}"
OS="${KRIT_OS:?}"
ARCH="${KRIT_ARCH:?}"
TAG="${KRIT_TAG:?}"
INSTALL_DIR="${KRIT_INSTALL_DIR:?}"

ARCHIVE_NAME="krit_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/kaeawc/krit/releases/download/${TAG}"
DOWNLOAD_URL="${BASE_URL}/${ARCHIVE_NAME}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"

echo "Downloading krit from $DOWNLOAD_URL"

mkdir -p "$INSTALL_DIR"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$DOWNLOAD_URL" -o "${TMPDIR}/${ARCHIVE_NAME}"

# Download checksums and verify (skip gracefully for dev builds)
if curl -fsSL "$CHECKSUMS_URL" -o "${TMPDIR}/checksums.txt" 2>/dev/null; then
    EXPECTED=$(grep "${ARCHIVE_NAME}" "${TMPDIR}/checksums.txt" | awk '{print $1}')
    if [ -z "$EXPECTED" ]; then
        echo "::warning::Archive ${ARCHIVE_NAME} not found in checksums.txt, skipping verification"
    else
        if command -v sha256sum > /dev/null 2>&1; then
            ACTUAL=$(sha256sum "${TMPDIR}/${ARCHIVE_NAME}" | awk '{print $1}')
        else
            ACTUAL=$(shasum -a 256 "${TMPDIR}/${ARCHIVE_NAME}" | awk '{print $1}')
        fi
        if [ "$EXPECTED" != "$ACTUAL" ]; then
            echo "::error::Checksum mismatch for ${ARCHIVE_NAME}: expected ${EXPECTED}, got ${ACTUAL}"
            exit 1
        fi
        echo "Checksum verified for ${ARCHIVE_NAME}"
    fi
else
    echo "::warning::checksums.txt not available, skipping verification (dev build?)"
fi

tar xz -C "$INSTALL_DIR" -f "${TMPDIR}/${ARCHIVE_NAME}"
chmod +x "$INSTALL_DIR/krit"
