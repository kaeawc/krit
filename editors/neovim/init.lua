-- Add to your init.lua or lua/plugins/krit.lua
local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

if not configs.krit then
  configs.krit = {
    default_config = {
      cmd = { 'krit-lsp' },
      filetypes = { 'kotlin' },
      root_dir = lspconfig.util.root_pattern('krit.yml', '.krit.yml', 'settings.gradle.kts', 'build.gradle.kts'),
      settings = {},
      init_options = {},
    },
  }
end

lspconfig.krit.setup({
  on_attach = function(client, bufnr)
    -- Optional: keymap for code actions
    vim.keymap.set('n', '<leader>ca', vim.lsp.buf.code_action, { buffer = bufnr })
  end,
})
