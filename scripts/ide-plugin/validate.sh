#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

(
  cd "${ROOT_DIR}/editors/intellij-plugin"
  "${ROOT_DIR}/tools/krit-fir/gradlew" test buildPlugin verifyPlugin
)
