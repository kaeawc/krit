# Krit Editor Integrations

Editor configurations for running `krit-lsp` as a language server.

## Supported Editors

| Editor | Directory | Method |
|--------|-----------|--------|
| VS Code | [`editors/vscode/`](vscode/) | Extension with auto-detection |
| Neovim | [`editors/neovim/`](neovim/) | nvim-lspconfig custom server |
| IntelliJ IDEA | [`editors/intellij/`](intellij/) | Built-in LSP (2024.2+) or LSP4IJ plugin |

## Building the LSP Server

```bash
go build -o krit-lsp ./cmd/krit-lsp/
```

Ensure the resulting binary is on your `$PATH`, or configure the absolute path in your editor.

## Configuration

All editors auto-detect `krit.yml` or `.krit.yml` from the project root. You can override this by passing a `configPath` in the LSP initialization options.
