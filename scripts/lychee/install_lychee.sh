#!/usr/bin/env bash
# Cross-platform lychee link checker installer.
set -euo pipefail

LYCHEE_VERSION="${LYCHEE_VERSION:-latest}"

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

if command_exists lychee; then
  echo "lychee is already installed: $(lychee --version)"
  exit 0
fi

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin)
    if command_exists brew; then
      echo "Installing lychee via Homebrew..."
      brew install lychee
    else
      echo "Homebrew not found. Install Homebrew first: https://brew.sh"
      exit 1
    fi
    ;;
  Linux)
    if command_exists brew; then
      echo "Installing lychee via Homebrew..."
      brew install lychee
    elif command_exists cargo; then
      echo "Installing lychee via cargo..."
      cargo install lychee
    else
      echo "Installing lychee from GitHub releases..."
      case "$ARCH" in
        x86_64) ARCH_SUFFIX="x86_64-unknown-linux-gnu" ;;
        aarch64|arm64) ARCH_SUFFIX="aarch64-unknown-linux-gnu" ;;
        *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
      esac

      if [[ "$LYCHEE_VERSION" == "latest" ]]; then
        DOWNLOAD_URL="https://github.com/lycheeverse/lychee/releases/latest/download/lychee-${ARCH_SUFFIX}.tar.gz"
      else
        DOWNLOAD_URL="https://github.com/lycheeverse/lychee/releases/download/v${LYCHEE_VERSION}/lychee-${ARCH_SUFFIX}.tar.gz"
      fi

      TMPDIR="$(mktemp -d)"
      trap 'rm -rf "$TMPDIR"' EXIT

      echo "Downloading $DOWNLOAD_URL"
      curl -fsSL "$DOWNLOAD_URL" -o "$TMPDIR/lychee.tar.gz"
      tar -xzf "$TMPDIR/lychee.tar.gz" -C "$TMPDIR"
      chmod +x "$TMPDIR/lychee"
      sudo mv "$TMPDIR/lychee" /usr/local/bin/lychee
    fi
    ;;
  MINGW*|MSYS*|CYGWIN*)
    if command_exists scoop; then
      echo "Installing lychee via Scoop..."
      scoop install lychee
    elif command_exists cargo; then
      echo "Installing lychee via cargo..."
      cargo install lychee
    else
      echo "Install Scoop (https://scoop.sh) or Rust/Cargo, then retry."
      exit 1
    fi
    ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

echo "lychee installed: $(lychee --version)"
