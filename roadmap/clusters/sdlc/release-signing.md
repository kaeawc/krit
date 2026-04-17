# Binary Signing and Release Hardening

**Cluster:** [sdlc](./README.md) · **Status:** in-progress ·
**Supersedes:** [`roadmap/26-binary-signing-and-release.md`](../../26-binary-signing-and-release.md)

## What it is

Code signing, checksum verification, and supply chain hardening for krit
binary distribution across macOS, Windows, and Linux.

## Current state (shipped)

- GoReleaser with version-stamped archive names
- SBOM generation via syft (CycloneDX)
- SLSA L3 provenance attestation via GitHub Actions
- Homebrew tap (`kaeawc/homebrew-tap`)
- Scoop bucket
- GPG signing of checksums (config ready, needs GPG key)

## Remaining work

- **Checksum verification in consumers:** GitHub Action, Gradle plugin,
  VS Code extension need to verify SHA-256 before extraction
- **macOS code signing + notarization:** Requires Apple Developer account
  ($99/yr). Eliminates Gatekeeper quarantine.
- **Windows Authenticode signing:** Requires code signing certificate
  ($200-500/yr). Eliminates SmartScreen warning.
- **Pin GitHub Actions by SHA** instead of tag

## Blockers

macOS signing needs Apple Developer Program enrollment.
Windows signing needs certificate purchase (Azure Trusted Signing is $10/mo alternative).
