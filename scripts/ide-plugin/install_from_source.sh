#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PLUGIN_PROJECT_DIR="${ROOT_DIR}/editors/intellij-plugin"
GRADLEW="${ROOT_DIR}/tools/krit-fir/gradlew"
RESTART_IDE=true

usage() {
  cat << EOF
Usage: $0 [--no-restart]

Build and install the Krit IntelliJ plugin from source.

Environment:
  KRIT_IDEA_PLUGINS_DIR   Explicit target plugins directory.
  IDEA_PLUGINS_DIR        IntelliJ plugins directory.
  ANDROID_STUDIO_PLUGINS_DIR
                          Android Studio plugins directory.
  IDE_APP_NAME            macOS app name to restart, e.g. "IntelliJ IDEA".
  IDE_CMD                 Linux/Windows IDE launch command.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-restart)
      RESTART_IDE=false
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

detect_plugins_dir_macos() {
  local candidates=()
  local jetbrains_dir="${HOME}/Library/Application Support/JetBrains"
  local google_dir="${HOME}/Library/Application Support/Google"

  if [[ -d "${jetbrains_dir}" ]]; then
    while IFS= read -r dir; do
      candidates+=("${dir}")
    done < <(find "${jetbrains_dir}" -maxdepth 1 -type d \
      \( -name "IntelliJIdea*" -o -name "IdeaIC*" -o -name "AndroidStudio*" \) 2>/dev/null)
  fi

  if [[ -d "${google_dir}" ]]; then
    while IFS= read -r dir; do
      candidates+=("${dir}")
    done < <(find "${google_dir}" -maxdepth 1 -type d -name "AndroidStudio*" 2>/dev/null)
  fi

  if [[ "${#candidates[@]}" -gt 0 ]]; then
    printf '%s\n' "${candidates[@]}" | sort -r | head -n 1 | sed 's#$#/plugins#'
  fi
}

IDE_PLUGIN_DIR="${KRIT_IDEA_PLUGINS_DIR:-${IDEA_PLUGINS_DIR:-${ANDROID_STUDIO_PLUGINS_DIR:-}}}"
OS_NAME="$(uname -s | tr '[:upper:]' '[:lower:]')"

if [[ -z "${IDE_PLUGIN_DIR}" && "${OS_NAME}" == "darwin" ]]; then
  IDE_PLUGIN_DIR="$(detect_plugins_dir_macos || true)"
fi

if [[ -z "${IDE_PLUGIN_DIR}" ]]; then
  echo "Could not auto-detect an IDE plugins directory."
  echo "Set KRIT_IDEA_PLUGINS_DIR, IDEA_PLUGINS_DIR, or ANDROID_STUDIO_PLUGINS_DIR."
  echo "Example:"
  echo "  export IDEA_PLUGINS_DIR=\"\$HOME/Library/Application Support/JetBrains/IntelliJIdea2025.3/plugins\""
  exit 1
fi

mkdir -p "${IDE_PLUGIN_DIR}"

echo "Using plugins directory: ${IDE_PLUGIN_DIR}"
echo "Building Krit IntelliJ plugin..."
(
  cd "${PLUGIN_PROJECT_DIR}"
  "${GRADLEW}" buildPlugin
)

PLUGIN_ZIP="$(find "${PLUGIN_PROJECT_DIR}/build/distributions" -maxdepth 1 -name '*.zip' -print0 2>/dev/null | xargs -0 ls -t 2>/dev/null | head -n 1 || true)"
if [[ -z "${PLUGIN_ZIP}" ]]; then
  echo "No plugin zip found in ${PLUGIN_PROJECT_DIR}/build/distributions" >&2
  exit 1
fi

PLUGIN_NAME="$(unzip -Z -1 "${PLUGIN_ZIP}" | head -n 1 | cut -d / -f 1)"
if [[ -z "${PLUGIN_NAME}" ]]; then
  PLUGIN_NAME="krit-intellij-plugin"
fi

echo "Installing ${PLUGIN_NAME}..."
rm -rf "${IDE_PLUGIN_DIR:?}/${PLUGIN_NAME:?}"
unzip -q "${PLUGIN_ZIP}" -d "${IDE_PLUGIN_DIR}"

echo "Installed ${PLUGIN_NAME} to ${IDE_PLUGIN_DIR}/${PLUGIN_NAME}"

select_from_list() {
  local prompt="$1"
  shift
  local options=("$@")
  local count="${#options[@]}"

  if [[ "${count}" -eq 0 || ! -t 0 ]]; then
    return 1
  fi

  echo "${prompt}"
  local i=1
  for option in "${options[@]}"; do
    echo "  [${i}] ${option}"
    i=$((i + 1))
  done

  read -r -p "Choose an option (1-${count}): " selection
  if [[ -z "${selection}" ]] || ! [[ "${selection}" =~ ^[0-9]+$ ]]; then
    return 1
  fi
  if (( selection < 1 || selection > count )); then
    return 1
  fi

  echo "${options[$((selection - 1))]}"
}

restart_ide_macos() {
  local app_name="$1"
  if [[ -z "${app_name}" ]]; then
    local known_apps=("IntelliJ IDEA" "IntelliJ IDEA Ultimate" "IntelliJ IDEA Community" "Android Studio" "Android Studio Preview")
    local running
    running="$(osascript -e 'tell application "System Events" to get name of (processes whose background only is false)' 2>/dev/null || true)"
    local matches=()
    for app in "${known_apps[@]}"; do
      if echo "${running}" | tr ',' '\n' | sed 's/^ *//;s/ *$//' | grep -Fxq "${app}"; then
        matches+=("${app}")
      fi
    done

    if [[ "${#matches[@]}" -eq 1 ]]; then
      app_name="${matches[0]}"
    elif [[ "${#matches[@]}" -gt 1 ]]; then
      app_name="$(select_from_list "Multiple IDEs are running. Which should be restarted?" "${matches[@]}" || true)"
    fi
  fi

  if [[ -z "${app_name}" ]]; then
    echo "Plugin installed. Restart your IDE manually to load it."
    return 0
  fi

  echo "Restarting ${app_name}..."
  osascript -e "tell application \"${app_name}\" to quit" 2>/dev/null || true
  sleep 3
  if pgrep -f "${app_name}.app/Contents/MacOS" >/dev/null 2>&1; then
    pkill -f "${app_name}.app/Contents/MacOS" 2>/dev/null || true
    sleep 1
  fi
  open -a "${app_name}"
}

restart_ide_linux() {
  local ide_cmd="${IDE_CMD:-}"
  if [[ -z "${ide_cmd}" ]]; then
    echo "Plugin installed. Set IDE_CMD or restart your IDE manually to load it."
    return 0
  fi

  echo "Restarting IDE via: ${ide_cmd}"
  pkill -f "studio|idea|intellij|android-studio" || true
  nohup "${ide_cmd}" >/dev/null 2>&1 &
}

restart_ide_windows() {
  local ide_cmd="${IDE_CMD:-}"
  if [[ -z "${ide_cmd}" ]]; then
    echo "Plugin installed. Set IDE_CMD or restart your IDE manually to load it."
    return 0
  fi

  echo "Restarting IDE via: ${ide_cmd}"
  taskkill //F //IM idea64.exe 2>/dev/null || true
  taskkill //F //IM studio64.exe 2>/dev/null || true
  cmd.exe /C start "" "${ide_cmd}"
}

if [[ "${RESTART_IDE}" != "true" ]]; then
  echo "Plugin installed. Restart skipped by --no-restart."
  exit 0
fi

case "${OS_NAME}" in
  darwin)
    restart_ide_macos "${IDE_APP_NAME:-}"
    ;;
  linux)
    restart_ide_linux
    ;;
  *)
    restart_ide_windows
    ;;
esac
