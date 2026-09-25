# Scoop bucket for krit

Krit publishes a Scoop manifest to the
[`kaeawc/scoop-krit`](https://github.com/kaeawc/scoop-krit) bucket on every
stable `vX.Y.Z` release. The `release` job in
`.github/workflows/release.yml` runs `scripts/release/update-scoop-bucket.sh`,
which renders `bucket/krit.json` with the release's Windows zip URL and
SHA256 (read from the combined `checksums.txt`) and pushes it to the
bucket. Prerelease tags (nightlies, release candidates) skip this step.

## Install

```powershell
scoop bucket add krit https://github.com/kaeawc/scoop-krit
scoop install krit
```

## Bucket setup

The release workflow needs a `SCOOP_BUCKET_TOKEN` repository secret
holding a PAT with Contents: write on `kaeawc/scoop-krit`.

## Manual publish

If the release step failed after the GitHub release was created, rerun
the script against that release's assets:

```bash
mkdir dist
gh release download vX.Y.Z --repo kaeawc/krit --pattern checksums.txt --dir dist
GH_TOKEN=<bucket PAT> TAG=vX.Y.Z REPO=kaeawc/krit bash scripts/release/update-scoop-bucket.sh
```
