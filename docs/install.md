# Install

## Recommended

The fastest path on macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/install.sh | sh
```

This downloads the latest release archive matching your OS/architecture,
verifies its SHA256 against the signed `checksums.txt`, and installs
`krit`, `krit-lsp`, and `krit-mcp` into `~/.local/bin`. Pass
`--dir <path>` to install elsewhere, or `--version v<tag>` to pin a
specific release.

The installer detects musl libc (Alpine, etc.) on `linux/amd64` and
fetches the appropriate static-linked archive. `linux/arm64-musl` and
`windows/arm64` are not currently published; build from source with
`go install` on those platforms.

## Homebrew (macOS / Linux)

```bash
brew install --cask kaeawc/tap/krit
```

Drops `krit`, `krit-lsp`, and `krit-mcp` into your Homebrew prefix.
Released as a cask (prebuilt binaries) rather than a formula because
krit requires CGO for tree-sitter — building from source on every
install would be slow and require a C toolchain.

## Scoop (Windows)

```powershell
scoop bucket add krit https://github.com/kaeawc/scoop-krit
scoop install krit
```

## winget (Windows)

```powershell
winget install kaeawc.krit
```

## Go

```bash
go install github.com/kaeawc/krit/cmd/krit@latest
go install github.com/kaeawc/krit/cmd/krit-lsp@latest
go install github.com/kaeawc/krit/cmd/krit-mcp@latest
```

Requires Go 1.25+ and a C compiler (CGO is required by tree-sitter).

## From source

```bash
git clone https://github.com/kaeawc/krit.git && cd krit
make build
```

CGO requires a C compiler. Optional compiler-backed analysis uses JVM
helper tools in `tools/krit-types/` and `tools/krit-fir/`; install a
JDK when you want KAA/FIR-backed checks.

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

- Installer / Go / source: `rm "$(which krit)" "$(which krit-lsp)" "$(which krit-mcp)"`
- Homebrew: `brew uninstall --cask kaeawc/tap/krit`
- Scoop: `scoop uninstall krit`
- winget: `winget uninstall kaeawc.krit`
