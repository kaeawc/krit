#!/usr/bin/env sh
# krit installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/install.sh | sh -s -- --version v0.1.0
#   curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/install.sh | sh -s -- --dir /usr/local/bin
#
# Environment variables (overridden by flags):
#   KRIT_VERSION    Tag to install. Defaults to the latest GitHub release.
#   KRIT_INSTALL_DIR  Install prefix. Defaults to "$HOME/.local/bin".
#   KRIT_REPO       owner/repo to fetch from. Defaults to "kaeawc/krit".
#
# What it installs: krit, krit-lsp, krit-mcp.
# Verification: SHA256 from the release's checksums.txt is checked
# before extraction. The script fails closed if any checksum doesn't match.

set -eu

# --- Defaults ----------------------------------------------------------------

KRIT_REPO="${KRIT_REPO:-kaeawc/krit}"
KRIT_VERSION="${KRIT_VERSION:-}"
KRIT_INSTALL_DIR="${KRIT_INSTALL_DIR:-$HOME/.local/bin}"

# --- Flags -------------------------------------------------------------------

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) KRIT_VERSION="$2"; shift 2 ;;
    --version=*) KRIT_VERSION="${1#*=}"; shift ;;
    --dir) KRIT_INSTALL_DIR="$2"; shift 2 ;;
    --dir=*) KRIT_INSTALL_DIR="${1#*=}"; shift ;;
    -h|--help)
      sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "krit install: unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# --- Helpers -----------------------------------------------------------------

err() { printf 'krit install: %s\n' "$*" >&2; exit 1; }
log() { printf 'krit install: %s\n' "$*"; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"
}

# Try to fetch a URL to stdout. Prefer curl; fall back to wget.
fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    err "need curl or wget"
  fi
}

# --- Detect platform ---------------------------------------------------------

os="$(uname -s)"
case "$os" in
  Linux)   goos=linux ;;
  Darwin)  goos=darwin ;;
  MINGW*|MSYS*|CYGWIN*) goos=windows ;;
  *) err "unsupported OS: $os (linux, macOS, and Windows-via-Git-Bash are supported)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) goarch=amd64 ;;
  arm64|aarch64) goarch=arm64 ;;
  *) err "unsupported architecture: $arch (amd64 and arm64 are supported)" ;;
esac

# Detect musl libc on Linux (Alpine, Void, etc.) so we fetch the
# static-linked musl archive instead of the glibc one. arm64-musl
# isn't shipped yet — fall back to glibc with a warning since it's
# unlikely to run.
libc=glibc
if [ "$goos" = linux ] && ldd /bin/ls 2>&1 | grep -qi musl; then
  if [ "$goarch" = amd64 ]; then
    libc=musl
  else
    log "musl libc detected on $goarch but only linux/musl/amd64 builds are published; falling back to glibc archive (may not run)"
  fi
fi

# Currently published archives:
#   linux/amd64, linux/arm64, linux/musl/amd64,
#   darwin/amd64, darwin/arm64, windows/amd64
# Anything else is a follow-up; bail with a build-from-source pointer.
case "${goos}/${goarch}" in
  linux/amd64|linux/arm64|darwin/amd64|darwin/arm64|windows/amd64) ;;
  *)
    err "${goos}/${goarch} archives are not currently published; build from source with 'go install github.com/kaeawc/krit/cmd/krit@latest'"
    ;;
esac

# --- Resolve version ---------------------------------------------------------

if [ -z "$KRIT_VERSION" ]; then
  log "fetching latest release tag from $KRIT_REPO"
  KRIT_VERSION="$(fetch "https://api.github.com/repos/$KRIT_REPO/releases/latest" \
    | grep -m1 '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  [ -n "$KRIT_VERSION" ] || err "could not determine latest release tag"
fi

# Strip the leading 'v' for archive naming; goreleaser archives use the
# version without the v.
ver="${KRIT_VERSION#v}"

# --- Compute archive name ----------------------------------------------------

if [ "$goos" = windows ]; then
  archive="krit_${ver}_${goos}_${goarch}.zip"
elif [ "$goos" = linux ] && [ "$libc" = musl ]; then
  archive="krit_${ver}_linux_musl_${goarch}.tar.gz"
else
  archive="krit_${ver}_${goos}_${goarch}.tar.gz"
fi

base="https://github.com/$KRIT_REPO/releases/download/$KRIT_VERSION"

# --- Stage in a temp dir -----------------------------------------------------

need uname
need mkdir
need rm

tmp="$(mktemp -d 2>/dev/null || mktemp -d -t krit-install)"
trap 'rm -rf "$tmp"' EXIT INT TERM

log "downloading $archive"
fetch "$base/$archive" > "$tmp/$archive"

log "downloading checksums.txt"
fetch "$base/checksums.txt" > "$tmp/checksums.txt"

# --- Verify checksum ---------------------------------------------------------

want="$(grep " $archive\$" "$tmp/checksums.txt" | awk '{print $1}')"
[ -n "$want" ] || err "no checksum entry for $archive in checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$tmp/$archive" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  got="$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')"
else
  err "need sha256sum or shasum to verify the archive"
fi

if [ "$want" != "$got" ]; then
  err "checksum mismatch for $archive: want $want, got $got"
fi
log "verified sha256: $got"

# --- Extract -----------------------------------------------------------------

cd "$tmp"
case "$archive" in
  *.tar.gz) need tar; tar -xzf "$archive" ;;
  *.zip)    need unzip; unzip -q "$archive" ;;
  *) err "unknown archive format: $archive" ;;
esac

# --- Install -----------------------------------------------------------------

mkdir -p "$KRIT_INSTALL_DIR"

for bin in krit krit-lsp krit-mcp; do
  src="$tmp/$bin"
  if [ "$goos" = windows ]; then
    src="$src.exe"
  fi
  if [ ! -f "$src" ]; then
    err "expected $src in archive; archive may be malformed"
  fi
  dest="$KRIT_INSTALL_DIR/$(basename "$src")"
  install -m 0755 "$src" "$dest" 2>/dev/null || cp "$src" "$dest"
  chmod +x "$dest" 2>/dev/null || true
  log "installed $dest"
done

# --- PATH check --------------------------------------------------------------

case ":$PATH:" in
  *":$KRIT_INSTALL_DIR:"*) ;;
  *)
    log "warning: $KRIT_INSTALL_DIR is not on your PATH"
    log "  add this to your shell rc: export PATH=\"$KRIT_INSTALL_DIR:\$PATH\""
    ;;
esac

log "done. krit $KRIT_VERSION installed."
log "run 'krit --version' to confirm."
