# Krit

Fast Kotlin static analysis powered by tree-sitter.

---

**472 rules** | **142 auto-fixable** | **single-pass AST dispatch** | **JSON / SARIF / Checkstyle output**

---

## Get started

```bash
brew install kaeawc/tap/krit   # install
krit --init                     # write a starter krit.yml
krit .                          # analyze the current directory
```

## Features

- **Single-pass analysis** — walks the AST once, dispatching to all matching rules
- **detekt-compatible** — 230 rules with matching names and config format
- **Android Lint–compatible** — 181 rules covering manifests, resources, icons, and Gradle
- **Auto-fix** — 142 rules produce ktfmt-compatible fixes with declared safety levels
- **Cross-file dead code detection** — indexes Kotlin, Java, and XML references with bloom filter lookups
- **Suppression** — `@Suppress("RuleName")` on any declaration, zero extra cost
- **Editor integration** — LSP server (11 capabilities), MCP server for AI agents, plus VS Code, Neovim, and IntelliJ configs

## Next steps

- [Install](install.md) — macOS, Linux, Windows, Go, Homebrew, Scoop
- [Quickstart](quickstart.md) — first scan in under a minute
- [Configuration](configuration.md) — `krit.yml` reference
- [Rules](rules.md) — full rule catalog
- [Integrations](integrations.md) — CI, editors, and MCP
