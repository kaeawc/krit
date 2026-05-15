#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PLUGIN_DIR="${ROOT_DIR}/editors/intellij-plugin"
POLL_INTERVAL=2
NO_RESTART=false

usage() {
  cat << EOF
Usage: $0 [--poll-interval seconds] [--no-restart]

Watch the Krit IntelliJ plugin sources. On change, rebuild and reinstall the
plugin using scripts/ide-plugin/install_from_source.sh.

This is a rebuild/reinstall loop, not IntelliJ dynamic plugin reload. Restarting
the IDE is the reliable path for extension-point changes.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --poll-interval)
      if [[ $# -lt 2 ]]; then
        echo "--poll-interval requires a value" >&2
        exit 1
      fi
      POLL_INTERVAL="$2"
      shift 2
      ;;
    --no-restart)
      NO_RESTART=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

hash_stream() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
  else
    sha256sum | awk '{print $1}'
  fi
}

stat_entry() {
  local file="$1"
  if stat -f '%N:%m:%z' "${file}" >/dev/null 2>&1; then
    stat -f '%N:%m:%z' "${file}"
  else
    stat -c '%n:%Y:%s' "${file}"
  fi
}

list_watch_files() {
  if command -v rg >/dev/null 2>&1; then
    rg --files "${PLUGIN_DIR}/src" -g '!**/build/**' 2>/dev/null || true
  else
    find "${PLUGIN_DIR}/src" -type f ! -path "*/build/*" 2>/dev/null || true
  fi

  for file in "${PLUGIN_DIR}/build.gradle.kts" "${PLUGIN_DIR}/settings.gradle.kts"; do
    if [[ -f "${file}" ]]; then
      echo "${file}"
    fi
  done
}

hash_state() {
  list_watch_files | while read -r file; do
    if [[ -f "${file}" ]]; then
      stat_entry "${file}" 2>/dev/null || true
    fi
  done | sort | hash_stream
}

install_once() {
  local args=()
  if [[ "${NO_RESTART}" == "true" ]]; then
    args+=(--no-restart)
  fi
  "${ROOT_DIR}/scripts/ide-plugin/install_from_source.sh" "${args[@]}"
}

echo "Watching ${PLUGIN_DIR} every ${POLL_INTERVAL}s..."
install_once
last_hash="$(hash_state)"

while true; do
  sleep "${POLL_INTERVAL}"
  next_hash="$(hash_state)"
  if [[ "${next_hash}" != "${last_hash}" ]]; then
    echo "Change detected. Rebuilding and reinstalling Krit IntelliJ plugin..."
    last_hash="${next_hash}"
    if install_once; then
      last_hash="$(hash_state)"
    else
      echo "Install failed; waiting for next change." >&2
    fi
  fi
done
