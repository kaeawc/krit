# Krit

Fast Kotlin static analysis in Go. 472 rules, tree-sitter parsing, no JVM.

```bash
brew install kaeawc/tap/krit
krit .
```

That's it. Run it in any Kotlin or Android project and get findings in seconds.

## What it does

- **472 rules** — detekt-compatible, Android Lint–compatible, plus extras for resources, icons, and Gradle
- **142 auto-fixes** at three safety levels — cosmetic, idiomatic, semantic
- **SARIF / JSON / Checkstyle / plain** output
- **LSP + MCP servers** — editor diagnostics and AI agent integration
- **Cross-file dead code** via bloom-filtered indexing across Kotlin, Java, and XML
- **Sub-second warm** on any cached project; 1.5s warm / 3.9s cold on Signal-Android (2,468 files, no KAA)

## Install

```bash
# macOS / Linux
brew install kaeawc/tap/krit

# Go
go install github.com/kaeawc/krit/cmd/krit@latest

# Script (macOS / Linux)
curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/scripts/install.sh | bash
```

See [docs/install.md](docs/install.md) for Windows, binary downloads, and companion tools (`krit-lsp`, `krit-mcp`).

## First run

```bash
krit --init     # write starter krit.yml
krit .          # analyze current directory
krit --fix .    # apply safe fixes
```

## Docs

- [Install](docs/install.md) — all install methods and companion binaries
- [Quickstart](docs/quickstart.md) — common commands and workflows
- [Configuration](docs/configuration.md) — `krit.yml`, thresholds, baselines
- [Rules](docs/rules.md) — all 472 rules by category
- [Integrations](docs/integrations.md) — IDE, CI, and MCP setup

## Contributing

```bash
git clone https://github.com/kaeawc/krit.git && cd krit
go build -o krit ./cmd/krit/
go test ./... -count=1
```

Rules live in `internal/rules/`. Fixtures in `tests/fixtures/`. New rules implement `DispatchRule` or `LineRule`. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

See [LICENSE](LICENSE).
