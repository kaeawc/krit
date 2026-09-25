# Changelog

## Unreleased

### Fixed

- Binary download now fetches the `krit_<version>_<os>_<arch>` release
  archive (`.zip` on Windows, the musl build on Alpine), verifies it against
  the release's `checksums.txt`, and extracts `krit-lsp`. Previously it
  requested a raw `krit-lsp-<os>-<arch>` binary that releases don't publish.
- `krit.version: latest` now resolves the latest release tag; it previously
  produced an invalid download URL. `krit.version` accepts `0.2.0` or
  `v0.2.0`.

## 0.1.0 (2026-04-08)

### Initial Release

- Language client connecting to krit-lsp via stdio
- Automatic binary detection in common install locations ($HOME/.krit/bin, $GOPATH/bin, /usr/local/bin)
- Binary download prompt when krit-lsp is not found locally
- Configuration support: enable/disable, custom binary path, config file path, version selection
- Status bar indicator showing Krit lint status
- File watcher for krit.yml configuration changes
- Activation on Kotlin file open
