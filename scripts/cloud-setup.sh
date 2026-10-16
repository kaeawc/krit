#!/bin/bash
# Environment setup script for Krit Claude Code cloud sessions.
#
# Paste into the cloud environment's "Setup script" setting (or run
# `bash scripts/cloud-setup.sh` from the repo root). Idempotent.
#
# Optional env vars (set in the environment settings):
#   KRIT_SETUP_JVM_JARS=1  prebuild the krit-fir / krit-types shadow jars
#                          (make fir-jar types-jar; slow, pulls Kotlin deps)
#   KRIT_SETUP_DOCS=1      install mkdocs + mkdocs-material for docs builds
set -eo pipefail
step() { echo "==> $*"; }
warn() { echo "WARN: $*" >&2; }

SUDO=""; [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null && SUDO="sudo"
export DEBIAN_FRONTEND=noninteractive
# Without a UTF-8 locale the JVM's sun.jnu.encoding is ASCII, and kotlinc in
# tools/krit-* can crash writing class files for non-ASCII test names.
export LANG=C.UTF-8 LC_ALL=C.UTF-8

REPO_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"
case "$(uname -m)" in
  x86_64|amd64) GOARCH_DL=amd64 ;;
  aarch64|arm64) GOARCH_DL=arm64 ;;
  *) GOARCH_DL="$(uname -m)" ;;
esac

# Returns success when version $1 >= version $2.
version_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -1)" = "$2" ]; }

step "apt packages"
$SUDO apt-get update -qq
$SUDO apt-get install -y -qq apt-utils >/dev/null 2>&1 || true
# build-essential: cgo tree-sitter grammars. jq: .claude PreToolUse hook.
# procps: scripts/go-test.sh counts processes with ps for its ulimit cap.
# The shellcheck package is needed by scripts/lint-actions.sh (via actionlint).
$SUDO apt-get install -y -qq --no-install-recommends \
  ca-certificates curl git jq unzip python3 procps build-essential shellcheck >/dev/null
if ! $SUDO apt-get install -y -qq --no-install-recommends openjdk-21-jdk-headless >/dev/null 2>&1; then
  warn "openjdk-21 not in apt; tools/krit-fir and tools/krit-types (jvmToolchain(21)) will not build"
fi

step "go (version from go.mod)"
GO_WANT=1.27.1
[ -f "$REPO_DIR/go.mod" ] && GO_WANT="$(awk '/^go /{print $2; exit}' "$REPO_DIR/go.mod")"
export PATH="/usr/local/go/bin:$PATH"
# GOTOOLCHAIN=local: under the default "auto", go inside the repo silently
# downloads and reports the go.mod toolchain, hiding an older installed Go.
export GOTOOLCHAIN=local
GO_HAVE="$(go env GOVERSION 2>/dev/null | sed 's/^go//' || true)"
if [ -z "$GO_HAVE" ] || ! version_ge "$GO_HAVE" "$GO_WANT"; then
  step "installing go $GO_WANT (found: ${GO_HAVE:-none})"
  curl -fsSL -o /tmp/go.tgz "https://go.dev/dl/go${GO_WANT}.linux-${GOARCH_DL}.tar.gz"
  $SUDO rm -rf /usr/local/go
  $SUDO tar -C /usr/local -xzf /tmp/go.tgz
  rm -f /tmp/go.tgz
fi
GOBIN_DIR="$(go env GOPATH)/bin"
export PATH="/usr/local/go/bin:$GOBIN_DIR:$PATH"
go version

# The image may ship a golangci-lint built with an older Go, which refuses to
# load this module ("Go language version used to build golangci-lint is lower
# than the targeted Go version"). Rebuild with the current toolchain, as CI
# does, and link over any stale copy in /usr/local/bin. GOTOOLCHAIN=local stays
# set so the tool's own go.mod cannot downgrade the build toolchain.
install_go_tool() {
  local name=$1 pkg=$2 bin="$GOBIN_DIR/$1" built
  built="$(go version "$bin" 2>/dev/null | awk '{print $2}' | sed 's/^go//' || true)"
  if [ -z "$built" ] || ! version_ge "$built" "$GO_WANT"; then
    step "installing $name"
    GOBIN="$GOBIN_DIR" go install "$pkg@latest"
  fi
  $SUDO ln -sf "$bin" "/usr/local/bin/$name"
}
install_go_tool golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint
install_go_tool actionlint github.com/rhysd/actionlint/cmd/actionlint
golangci-lint version
actionlint -version | head -1

step "gradle: retry Maven Central rate limits"
# repo.maven.apache.org answers HTTP 429 under Gradle's parallel download burst,
# which surfaces as misleading "plugin not found" errors. Retry with backoff.
mkdir -p "$HOME/.gradle"
touch "$HOME/.gradle/gradle.properties"
for prop in \
  "systemProp.org.gradle.internal.repository.max.retries=10" \
  "systemProp.org.gradle.internal.repository.initial.backoff=3000"; do
  grep -qxF "$prop" "$HOME/.gradle/gradle.properties" || echo "$prop" >> "$HOME/.gradle/gradle.properties"
done

step "shell env"
if ! grep -q '# >>> krit env >>>' "$HOME/.bashrc" 2>/dev/null; then
  cat >> "$HOME/.bashrc" <<EOF
# >>> krit env >>>
export LANG=C.UTF-8 LC_ALL=C.UTF-8
export PATH="/usr/local/go/bin:$GOBIN_DIR:\$PATH"
# <<< krit env <<<
EOF
fi

step "repo deps"
cd "$REPO_DIR"
if [ -f go.mod ] && grep -q '^module github.com/kaeawc/krit' go.mod; then
  go mod download
  # Warm the build cache (cgo tree-sitter grammars dominate a cold build).
  go build ./... || warn "go build ./... failed; the session will rebuild from scratch"
  if [ "${KRIT_SETUP_JVM_JARS:-0}" = 1 ]; then
    step "jvm helper jars"
    make fir-jar >/tmp/krit-fir-jar.log 2>&1 || warn "make fir-jar failed (see /tmp/krit-fir-jar.log)"
    make types-jar >/tmp/krit-types-jar.log 2>&1 || warn "make types-jar failed (see /tmp/krit-types-jar.log)"
  fi
  if [ "${KRIT_SETUP_DOCS:-0}" = 1 ]; then
    step "docs tooling"
    python3 -m pip install --quiet --user mkdocs mkdocs-material 2>/dev/null \
      || python3 -m pip install --quiet --user --break-system-packages mkdocs mkdocs-material \
      || warn "mkdocs install failed"
  fi
else
  step "krit go.mod not found in $REPO_DIR; skipping repo deps"
fi
step "done"
