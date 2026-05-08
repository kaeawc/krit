# Krit - Neovim Setup

## Prerequisites

- [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig)
- `krit-lsp` binary on your `$PATH` (build with `go build -o krit-lsp ./cmd/krit-lsp/`)

## Installation

Copy the contents of `init.lua` into your Neovim config (`~/.config/nvim/init.lua` or a dedicated plugin file like `lua/plugins/krit.lua`).

## Configuration

The LSP server auto-detects `krit.yml` or `.krit.yml` in your project root. To use a custom config path, pass it via `init_options`:

```lua
lspconfig.krit.setup({
  init_options = {
    configPath = '/path/to/krit.yml',
  },
})
```

## Keybindings

The default config maps `<leader>ca` to code actions. Diagnostics appear inline via Neovim's built-in diagnostic UI.
