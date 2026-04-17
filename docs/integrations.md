# Integrations

Krit ships three integration surfaces:

- **LSP** (`krit-lsp`) — editor diagnostics and quick fixes
- **MCP** (`krit-mcp`) — AI agent queries over analysis results
- **CI** — GitHub Action, Gradle plugin, and raw CLI for any runner

## Editors (LSP)

The `krit-lsp` binary implements the Language Server Protocol with 11 capabilities: diagnostics, code actions, formatting, hover, symbols, definition, references, rename, completion, incremental sync, and config reload.

```bash
go install github.com/kaeawc/krit/cmd/krit-lsp@latest
```

Editor configs live in `editors/`:

| Editor    | Path                 | How to install |
|-----------|----------------------|----------------|
| VS Code   | `editors/vscode/`    | `npm install && npm run package && code --install-extension krit-*.vsix` |
| Neovim    | `editors/neovim/`    | Copy the LSP config snippet into your `init.lua` or use `nvim-lspconfig` |
| IntelliJ  | `editors/intellij/`  | JetBrains plugin — install via the Plugin Marketplace or from disk |

All editors pick up `krit.yml` / `.krit.yml` from the workspace root. Troubleshoot LSP issues with:

```bash
which krit-lsp        # check PATH
krit-lsp --version    # verify install
```

Editor-side logs: VS Code "Output → Krit", Neovim `:LspLog`, IntelliJ "Help → Show Log".

## AI agents (MCP)

`krit-mcp` is a Model Context Protocol server for Claude Code, Cursor, Windsurf, and any MCP-compatible agent.

```json
{
  "mcpServers": {
    "krit": {
      "command": "krit-mcp",
      "args": ["--project", "."]
    }
  }
}
```

Drop this into `.claude/settings.json` (Claude Code) or `.cursor/mcp.json` (Cursor).

**Tools (8)**: `analyze`, `suggest_fixes`, `explain_rule`, `inspect_types`, `find_references`, `analyze_project`, `analyze_android`, `inspect_modules`

**Prompts (3)**: `review_kotlin`, `prepare_pr`, `refactor_check`

**Resources (2)**: `krit://rules`, `krit://schema`

Install:

```bash
go install github.com/kaeawc/krit/cmd/krit-mcp@latest
```

## GitHub Action

```yaml
name: Krit
on: [push, pull_request]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/krit-action/
        with:
          args: '--format sarif -o results.sarif .'
      - uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: results.sarif
```

PR-only check (fastest feedback):

```yaml
- uses: ./.github/actions/krit-action/
  with:
    diff: origin/main
```

Exit codes: `0` clean, `1` findings, `2` config error.

## Gradle plugin

```kotlin
plugins {
  id("io.github.kaeawc.krit") version "0.1.0"
}

krit {
  configFile.set(file("krit.yml"))
  format.set("sarif")
  outputFile.set(file("build/reports/krit.sarif"))
  baseline.set(file("baseline.xml"))
}
```

Tasks: `kritAnalyze`, `kritFix`, `kritBaseline`.

```bash
./gradlew kritAnalyze
./gradlew kritFix
./gradlew kritBaseline
```

Multi-module setup from the root `build.gradle.kts`:

```kotlin
subprojects {
  apply(plugin = "io.github.kaeawc.krit")
  krit { configFile.set(rootProject.file("krit.yml")) }
}
```

## Pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/kaeawc/krit
    rev: v0.1.0
    hooks:
      - id: krit
```

## Output formats for CI

| Format     | Use case                    | Flag |
|------------|-----------------------------|------|
| SARIF      | GitHub Code Scanning        | `--format sarif` |
| Checkstyle | Jenkins, other CI tools     | `--format checkstyle` |
| JSON       | Custom processing (default) | `--format json` |
| Plain      | Human-readable logs         | `--format plain` |
