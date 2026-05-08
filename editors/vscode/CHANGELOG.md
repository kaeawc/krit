# Changelog

## 0.1.0 (2026-04-08)

### Initial Release

- Language client connecting to krit-lsp via stdio
- Automatic binary detection in common install locations ($HOME/.krit/bin, $GOPATH/bin, /usr/local/bin)
- Binary download prompt when krit-lsp is not found locally
- Configuration support: enable/disable, custom binary path, config file path, version selection
- Status bar indicator showing Krit lint status
- File watcher for krit.yml configuration changes
- Activation on Kotlin file open
