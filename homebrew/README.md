# Homebrew tap for krit

Krit publishes a Homebrew **cask** to `kaeawc/homebrew-tap` automatically
on every tagged release via GoReleaser (the `homebrew_casks:` block in
`.goreleaser.yml`). There is no hand-written formula or cask checked into
this repo — GoReleaser generates and pushes the cask file with correct
download URLs and signed SHA256s.

## Install (after the first release is cut)

```bash
brew install --cask kaeawc/tap/krit
```

This drops `krit`, `krit-lsp`, and `krit-mcp` onto your `PATH`.

## One-time tap setup

The tap repo (`kaeawc/homebrew-tap`) needs to exist before the first
`v*` tag is pushed — it does, it's empty until GoReleaser populates it.
The release workflow also needs a `HOMEBREW_TAP_TOKEN` repository secret
holding a PAT with write access to the tap repo.

## Why a cask, not a formula

Krit ships prebuilt CGO binaries (tree-sitter requires CGO). A formula
would either need to compile from source on every install (slow,
requires a C toolchain on the user's machine) or ship binaries — which
is exactly what a cask does. Cask is the right tool for prebuilt-binary
distribution.

## Emergency manual publish

If the automated path is broken and you need to push a cask manually:

1. Build and sign archives for darwin amd64 + arm64.
2. Compute SHA256s.
3. Render a cask file (use a previous release's generated cask as a
   template) and commit it to `Casks/krit.rb` in the tap repo.

Under normal operation this should never be necessary; fix the release
workflow instead.
