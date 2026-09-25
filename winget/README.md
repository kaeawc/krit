# winget manifests for krit

Krit's winget package (`kaeawc.krit`) is generated on every stable
`vX.Y.Z` release; no manifest is checked into this repo.

1. The `release` job in `.github/workflows/release.yml` runs
   `scripts/release/render-winget-manifests.sh`, which writes the
   version, installer, and default-locale manifests (multi-file,
   `ManifestVersion` 1.6.0) for the release's Windows zip, with its
   SHA256 read from the combined `checksums.txt`. The zip installs
   `krit`, `krit-lsp`, and `krit-mcp` as portable commands. The rendered
   directory is uploaded as the run's `winget-manifests` artifact.
2. The `winget` job submits those manifests to
   [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs) with
   `wingetcreate submit`, which opens a PR from the token owner's fork.
   A version becomes installable once winget-pkgs merges that PR.

Prerelease tags (nightlies, release candidates) skip both steps.

## Install

```powershell
winget install kaeawc.krit
```

## Setup

The `winget` job needs a `WINGET_TOKEN` repository secret: a classic PAT
with `public_repo` scope. Without it the job logs a notice and does
nothing, leaving the manifests in the `winget-manifests` artifact.

## Manual submission

Download the `winget-manifests` artifact from the release run, or render
the manifests locally:

```bash
mkdir dist
gh release download vX.Y.Z --repo kaeawc/krit --pattern checksums.txt --dir dist
TAG=vX.Y.Z REPO=kaeawc/krit bash scripts/release/render-winget-manifests.sh
```

Then submit from Windows:

```powershell
wingetcreate submit --token <PAT> dist/winget/manifests/k/kaeawc/krit/X.Y.Z
```
