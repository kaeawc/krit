# Homebrew Tap for Krit

This directory contains the Homebrew formula template for Krit.

## Setting Up the Tap Repository

1. Create a new GitHub repository named `kaeawc/homebrew-tap`.

2. Copy the formula into the repo:
   ```bash
   git clone https://github.com/kaeawc/homebrew-tap.git
   cp homebrew/krit.rb homebrew-tap/Formula/krit.rb
   cd homebrew-tap
   git add Formula/krit.rb
   git commit -m "Add krit formula"
   git push origin main
   ```

3. Users can then install with:
   ```bash
   brew tap kaeawc/tap
   brew install krit
   ```

## Updating the Formula

Replace `PLACEHOLDER` sha256 values with actual checksums from the release artifacts:
```bash
shasum -a 256 krit_0.1.0_darwin_arm64.tar.gz
```

## GoReleaser Integration

GoReleaser can auto-update this formula on each release. Add to `.goreleaser.yaml`:
```yaml
brews:
  - repository:
      owner: kaeawc
      name: homebrew-tap
    folder: Formula
    homepage: "https://github.com/kaeawc/krit"
    description: "Fast Kotlin static analysis powered by tree-sitter"
    license: "MIT"
    install: |
      bin.install "krit"
      bin.install "krit-lsp"
      bin.install "krit-mcp"
    test: |
      system "#{bin}/krit", "--version"
```

With this configuration, GoReleaser will automatically push updated formulas to the tap repository on every release.
