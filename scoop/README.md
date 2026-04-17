# Scoop Bucket for Krit

This directory contains the Scoop manifest template for Krit.

## Setting Up the Bucket Repository

1. Create a new GitHub repository named `kaeawc/scoop-krit`.

2. Copy the manifest into the repo:
   ```bash
   git clone https://github.com/kaeawc/scoop-krit.git
   cp scoop/krit.json scoop-krit/bucket/krit.json
   cd scoop-krit
   git add bucket/krit.json
   git commit -m "Add krit manifest"
   git push origin main
   ```

3. Users can then install with:
   ```powershell
   scoop bucket add krit https://github.com/kaeawc/scoop-krit
   scoop install krit
   ```

## Updating the Manifest

Replace the `PLACEHOLDER` hash value with the actual SHA256 of the release zip:
```powershell
Get-FileHash krit_0.1.0_windows_amd64.zip -Algorithm SHA256
```

## GoReleaser Integration

GoReleaser can auto-update this manifest on each release. Add to `.goreleaser.yaml`:
```yaml
scoops:
  - repository:
      owner: kaeawc
      name: scoop-krit
    folder: bucket
    homepage: "https://github.com/kaeawc/krit"
    description: "Fast Kotlin static analysis powered by tree-sitter"
    license: MIT
```

With this configuration, GoReleaser will automatically push updated manifests to the bucket repository on every release.
