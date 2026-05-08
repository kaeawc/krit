# Install

## Go

```bash
go install github.com/kaeawc/krit/cmd/krit@latest
```

Requires Go 1.25+.

## From source

```bash
git clone https://github.com/kaeawc/krit.git && cd krit
make build
```

CGO is required for tree-sitter, so source builds need a C compiler. Optional
compiler-backed analysis uses JVM helper tools in `tools/krit-types/` and
`tools/krit-fir/`; install a JDK when you want KAA/FIR-backed checks.

## Binary releases

Pre-built binaries, Homebrew, Scoop, winget, and install-script flows should be
treated as release-channel features. Use Go/source installs when working from a
fresh checkout.

Use the Go or source install paths above for now.

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

- Go/binary: `rm "$(which krit)"`
