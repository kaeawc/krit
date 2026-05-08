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

3. After the first GitHub release is cut and real checksums are filled in,
   users can install with:
   ```bash
   brew tap kaeawc/tap
   brew install krit
   ```

## Updating the Formula

Replace `PLACEHOLDER` sha256 values with actual checksums from the release artifacts before publishing:
```bash
shasum -a 256 krit_0.1.0_darwin_arm64.tar.gz
```

## GoReleaser Integration

GoReleaser auto-publishes the formula to `kaeawc/homebrew-tap` on every
tagged release — see the `brews:` block in `.goreleaser.yml` at the repo
root. The manual formula in this directory is only a fallback template for
the first release or for emergency manual publishes; under normal operation
GoReleaser overwrites `Formula/krit.rb` in the tap repo with the correct
URLs and SHAs.

The release workflow needs a `HOMEBREW_TAP_TOKEN` secret with write access
to `kaeawc/homebrew-tap`.
