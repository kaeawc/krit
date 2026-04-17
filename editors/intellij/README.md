# Krit - IntelliJ IDEA Setup

## Prerequisites

- `krit-lsp` binary on your `$PATH` (build with `go build -o krit-lsp ./cmd/krit-lsp/`)
- IntelliJ IDEA 2024.2+ (built-in LSP support) **or** the [LSP4IJ](https://plugins.jetbrains.com/plugin/23257-lsp4ij) plugin for older versions

## Using IntelliJ 2024.2+ Built-in LSP

1. Open **Settings > Languages & Frameworks > Language Servers**
2. Click **+** to add a new server
3. Set the name to `krit`
4. Set the command to `krit-lsp`
5. Under **File Patterns**, add `*.kt` and `*.kts`
6. Click **Apply**

## Using LSP4IJ Plugin

1. Install [LSP4IJ](https://plugins.jetbrains.com/plugin/23257-lsp4ij) from the JetBrains Marketplace
2. Open **Settings > Languages & Frameworks > Language Server Protocol > Server Definitions**
3. Add a new **Executable** server definition:
   - **Name**: `krit`
   - **Command**: `krit-lsp`
   - **Language mappings**: `kotlin`

## Settings Snippet

If your editor tooling accepts JSON-based LSP configuration:

```json
{
  "lsp": {
    "krit": {
      "command": ["krit-lsp"],
      "languages": ["kotlin"],
      "initializationOptions": {
        "configPath": ""
      }
    }
  }
}
```

Leave `configPath` empty to auto-detect `krit.yml` / `.krit.yml` from the project root.
