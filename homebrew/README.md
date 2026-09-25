# Homebrew tap for krit

Krit publishes a Homebrew **cask** to `kaeawc/homebrew-tap` on every
stable `vX.Y.Z` release. The `release` job in
`.github/workflows/release.yml` runs `scripts/release/update-brew-tap.sh`,
which renders `Casks/krit.rb` with the release's darwin and linux archive
URLs and SHA256s (read from the combined `checksums.txt`) and pushes it to
the tap. GoReleaser does not publish the cask, and no formula or cask is
checked into this repo. Prerelease tags (nightlies, release candidates)
skip this step.

## Install

```bash
brew install --cask kaeawc/tap/krit
```

This drops `krit`, `krit-lsp`, and `krit-mcp` onto your `PATH`.

## Tap setup

The tap repo (`kaeawc/homebrew-tap`) must exist, and the release workflow
needs a `HOMEBREW_TAP_TOKEN` repository secret holding a PAT with
Contents: write on the tap repo.

## Why a cask, not a formula

Krit ships prebuilt CGO binaries (tree-sitter requires CGO). A formula
would either need to compile from source on every install (slow,
requires a C toolchain on the user's machine) or ship binaries — which
is exactly what a cask does. Cask is the right tool for prebuilt-binary
distribution.

## Manual publish

If the release step failed after the GitHub release was created, rerun
the script against that release's assets:

```bash
mkdir dist
gh release download vX.Y.Z --repo kaeawc/krit --pattern checksums.txt --dir dist
GH_TOKEN=<tap PAT> TAG=vX.Y.Z REPO=kaeawc/krit bash scripts/release/update-brew-tap.sh
```
