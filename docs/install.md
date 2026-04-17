# Install

## macOS / Linux

```bash
brew install kaeawc/tap/krit
```

Or use the installer script:

```bash
curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/scripts/install.sh | bash
```

Pin a version or choose a directory:

```bash
KRIT_VERSION=0.2.0 KRIT_INSTALL_DIR=~/.local/bin \
  curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/scripts/install.sh | bash
```

## Windows

```powershell
irm https://raw.githubusercontent.com/kaeawc/krit/main/scripts/install.ps1 | iex
```

Or via Scoop:

```powershell
scoop bucket add krit https://github.com/kaeawc/scoop-krit
scoop install krit
```

## Go

```bash
go install github.com/kaeawc/krit/cmd/krit@latest
```

Requires Go 1.22+.

## From source

```bash
git clone https://github.com/kaeawc/krit.git && cd krit
go build -o krit ./cmd/krit/
```

CGO is required for tree-sitter — ensure a C compiler is available.

## Binary releases

Pre-built binaries and `checksums.txt` live at [GitHub Releases](https://github.com/kaeawc/krit/releases). Verify with:

```bash
sha256sum -c checksums.txt --ignore-missing
```

## Companion binaries

Optional, same release channel:

```bash
go install github.com/kaeawc/krit/cmd/krit-lsp@latest   # editor LSP
go install github.com/kaeawc/krit/cmd/krit-mcp@latest   # AI agent MCP
```

## Shell completions

```bash
eval "$(krit --completions bash)"   # ~/.bashrc
eval "$(krit --completions zsh)"    # ~/.zshrc
krit --completions fish > ~/.config/fish/completions/krit.fish
```

Or run `make install-completions` from a source checkout.

## Verify

```bash
krit --version
```

## Uninstall

- Homebrew: `brew uninstall krit`
- Scoop: `scoop uninstall krit`
- Go/binary: `rm "$(which krit)"`
