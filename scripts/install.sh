#!/usr/bin/env bash
set -euo pipefail

VERSION="${KRIT_VERSION:-latest}"
INSTALL_DIR="${KRIT_INSTALL_DIR:-/usr/local/bin}"
REPO="kaeawc/krit"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${BLUE}[info]${NC} $1"; }
success() { echo -e "${GREEN}[ok]${NC} $1"; }
warn()    { echo -e "${YELLOW}[warn]${NC} $1"; }
error()   { echo -e "${RED}[error]${NC} $1"; exit 1; }

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------
detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux*)  os="linux" ;;
        darwin*) os="darwin" ;;
        mingw*|msys*|cygwin*) os="windows" ;;
        *) error "Unsupported OS: $os" ;;
    esac

    case "$arch" in
        x86_64)         arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        armv7l)         arch="armv7" ;;
        *) error "Unsupported architecture: $arch" ;;
    esac

    PLATFORM="${os}"
    ARCH="${arch}"
    PLATFORM_PAIR="${os}_${arch}"
}

# ---------------------------------------------------------------------------
# Check for / optionally install gum
# ---------------------------------------------------------------------------
HAS_GUM=false

ensure_gum() {
    if command -v gum &>/dev/null; then
        HAS_GUM=true
        return 0
    fi

    info "gum (charmbracelet/gum) enables prettier prompts."

    local install_gum="n"
    read -rp "Install gum for a nicer experience? [y/N] " install_gum
    case "$install_gum" in
        [yY]*)
            if command -v brew &>/dev/null; then
                brew install gum && HAS_GUM=true && return 0
            elif command -v go &>/dev/null; then
                go install github.com/charmbracelet/gum@latest && HAS_GUM=true && return 0
            else
                warn "Cannot auto-install gum (no brew or go). Continuing with plain prompts."
            fi
            ;;
    esac
    warn "Using plain prompts."
    return 0
}

# ---------------------------------------------------------------------------
# Interactive method chooser
# ---------------------------------------------------------------------------
choose_method() {
    local methods=()

    if command -v brew &>/dev/null; then
        methods+=("Homebrew  (brew install ${REPO/\//#tap#/})" )
        # Friendly label; we parse the prefix later
    fi
    if command -v scoop &>/dev/null; then
        methods+=("Scoop  (scoop install krit)")
    fi
    if command -v go &>/dev/null; then
        methods+=("Go install  (go install github.com/${REPO}/cmd/krit@latest)")
    fi
    methods+=("Download binary from GitHub Releases")

    local choice=""
    if $HAS_GUM; then
        choice="$(gum choose --header "How would you like to install krit?" "${methods[@]}")"
    else
        echo ""
        echo -e "${BOLD}How would you like to install krit?${NC}"
        for i in "${!methods[@]}"; do
            echo "  $((i + 1))) ${methods[$i]}"
        done
        local num=""
        read -rp "Enter choice [1-${#methods[@]}]: " num
        if [[ -z "$num" ]] || (( num < 1 || num > ${#methods[@]} )); then
            error "Invalid selection."
        fi
        choice="${methods[$((num - 1))]}"
    fi

    # Return the prefix keyword
    case "$choice" in
        Homebrew*)  echo "brew" ;;
        Scoop*)     echo "scoop" ;;
        "Go install"*) echo "go" ;;
        Download*)  echo "binary" ;;
        *)          error "Unknown selection: $choice" ;;
    esac
}

# ---------------------------------------------------------------------------
# Install methods
# ---------------------------------------------------------------------------
install_brew() {
    info "Installing via Homebrew..."
    brew install kaeawc/tap/krit
}

install_scoop() {
    info "Installing via Scoop..."
    scoop install krit
}

install_go() {
    info "Installing via go install..."
    if [ "$VERSION" = "latest" ]; then
        go install "github.com/${REPO}/cmd/krit@latest"
    else
        go install "github.com/${REPO}/cmd/krit@v${VERSION#v}"
    fi
}

install_binary() {
    info "Installing from GitHub Releases..."

    local tag="$VERSION"
    if [ "$tag" = "latest" ]; then
        tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' | sed 's/.*"tag_name": *"//' | sed 's/".*//')"
        if [ -z "$tag" ]; then
            error "Could not determine latest release tag."
        fi
        info "Latest release: ${tag}"
    fi

    local version="${tag#v}"
    local archive="krit_${version}_${PLATFORM_PAIR}.tar.gz"
    local base_url="https://github.com/${REPO}/releases/download/${tag}"
    local url="${base_url}/${archive}"

    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    info "Downloading ${archive}..."
    curl -fsSL "$url" -o "${tmpdir}/${archive}" || error "Download failed. Check that release ${tag} exists for ${PLATFORM_PAIR}."

    # Checksum verification
    local checksums_url="${base_url}/checksums.txt"
    if curl -fsSL "$checksums_url" -o "${tmpdir}/checksums.txt" 2>/dev/null; then
        local expected actual
        expected="$(grep "${archive}" "${tmpdir}/checksums.txt" | awk '{print $1}')"
        if [ -n "$expected" ]; then
            if command -v sha256sum &>/dev/null; then
                actual="$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')"
            else
                actual="$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')"
            fi
            if [ "$expected" != "$actual" ]; then
                error "Checksum mismatch! Expected ${expected}, got ${actual}."
            fi
            success "Checksum verified."
        else
            warn "Archive not found in checksums.txt; skipping verification."
        fi
    else
        warn "checksums.txt not available; skipping verification."
    fi

    # Extract
    mkdir -p "$INSTALL_DIR"
    tar xzf "${tmpdir}/${archive}" -C "${tmpdir}"
    install -m 755 "${tmpdir}/krit" "${INSTALL_DIR}/krit"
    success "Installed krit to ${INSTALL_DIR}/krit"
}

# ---------------------------------------------------------------------------
# Verify installation
# ---------------------------------------------------------------------------
verify_install() {
    echo ""
    if command -v krit &>/dev/null; then
        local ver
        ver="$(krit --version 2>&1 || true)"
        success "krit is installed: ${ver}"
    else
        warn "krit not found on PATH."
        warn "You may need to add ${INSTALL_DIR} to your PATH:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
}

# ---------------------------------------------------------------------------
# Optional companion tools
# ---------------------------------------------------------------------------
install_extras() {
    echo ""
    local answer=""
    if $HAS_GUM; then
        answer="$(gum choose --header "Install companion tools?" \
            "Yes, install krit-lsp and krit-mcp" \
            "Just krit-lsp" \
            "Just krit-mcp" \
            "No thanks")"
    else
        echo -e "${BOLD}Install companion tools?${NC}"
        echo "  1) Yes, install krit-lsp and krit-mcp"
        echo "  2) Just krit-lsp"
        echo "  3) Just krit-mcp"
        echo "  4) No thanks"
        read -rp "Enter choice [1-4]: " num
        case "$num" in
            1) answer="Yes, install krit-lsp and krit-mcp" ;;
            2) answer="Just krit-lsp" ;;
            3) answer="Just krit-mcp" ;;
            *) answer="No thanks" ;;
        esac
    fi

    local install_lsp=false install_mcp=false
    case "$answer" in
        "Yes, install krit-lsp and krit-mcp") install_lsp=true; install_mcp=true ;;
        "Just krit-lsp") install_lsp=true ;;
        "Just krit-mcp") install_mcp=true ;;
        *) return 0 ;;
    esac

    if $install_lsp; then
        info "Installing krit-lsp..."
        if command -v go &>/dev/null; then
            go install "github.com/${REPO}/cmd/krit-lsp@latest"
            success "krit-lsp installed."
        else
            warn "go not found; skipping krit-lsp (requires Go toolchain)."
        fi
    fi

    if $install_mcp; then
        info "Installing krit-mcp..."
        if command -v go &>/dev/null; then
            go install "github.com/${REPO}/cmd/krit-mcp@latest"
            success "krit-mcp installed."
        else
            warn "go not found; skipping krit-mcp (requires Go toolchain)."
        fi
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    echo ""
    echo -e "${BOLD}krit installer${NC}"
    echo -e "Repository: github.com/${REPO}"
    echo ""

    detect_platform
    info "Detected platform: ${PLATFORM_PAIR}"

    ensure_gum

    local method
    method="$(choose_method)"

    case "$method" in
        brew)   install_brew ;;
        scoop)  install_scoop ;;
        go)     install_go ;;
        binary) install_binary ;;
    esac

    verify_install
    install_extras

    echo ""
    success "All done! Run 'krit --help' to get started."
}

main "$@"
