#!/usr/bin/env bash
set -euo pipefail

# Downloads the krit binary for the current platform for
# .github/actions/krit-action. The download and verification are
# scripts/action/install-krit.sh's, shared with the root action.yml: it fails
# closed when checksums.txt is missing, does not list the archive, or does not
# match.
# Expected environment variables (set by the GitHub Action):
#   KRIT_VERSION_BARE  - version without leading v (e.g. 0.3.1)
#   KRIT_OS            - operating system (linux, darwin)
#   KRIT_ARCH          - architecture (amd64, arm64)
#   KRIT_TAG           - full tag (e.g. v0.3.1)
#   KRIT_INSTALL_DIR   - directory to install into

KRIT_ASSET="krit_${KRIT_VERSION_BARE:?}_${KRIT_OS:?}_${KRIT_ARCH:?}.tar.gz" \
  KRIT_TAG="${KRIT_TAG:?}" \
  KRIT_DIR="${KRIT_INSTALL_DIR:?}" \
  bash "$(dirname "${BASH_SOURCE[0]}")/action/install-krit.sh" install
chmod +x "$KRIT_INSTALL_DIR/krit"
