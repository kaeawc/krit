# Krit

Go-first static analysis for Kotlin, Java, and Android. Krit uses tree-sitter
for fast source analysis and can call JVM-backed Kotlin Analysis API/FIR helper
tools for type-aware checks when a project needs them.

```bash
go install github.com/kaeawc/krit/cmd/krit@latest
krit .
```

That's it. Run it in any Kotlin, Java, or Android project and get findings in seconds.

## What it does

- **Broad rule coverage** — detekt-compatible, Android Lint-compatible, plus extras for resources, icons, and Gradle
- **Auto-fixes** at three safety levels — cosmetic, idiomatic, semantic
- **SARIF / JSON / Checkstyle / plain** output
- **LSP + MCP servers** — editor diagnostics and AI agent integration
- **Cross-file dead code** via bloom-filtered indexing across Kotlin, Java, and XML
- **Optional JVM analysis** through `krit-types`/KAA and FIR helper processes for checks that need compiler-grade facts

## Install

```bash
# Go
go install github.com/kaeawc/krit/cmd/krit@latest

# From source
go build -o krit ./cmd/krit/
```

See [docs/install.md](docs/install.md) for current install options and companion tools (`krit-lsp`, `krit-mcp`, `krit-types`, and `krit-fir`).

## First run

```bash
krit --init     # write starter krit.yml
krit .          # analyze current directory
krit --fix .    # apply safe fixes
```

## Docs

- [Install](docs/install.md) — install options and companion binaries
- [Quickstart](docs/quickstart.md) — common commands and workflows
- [Configuration](docs/configuration.md) — `krit.yml`, thresholds, baselines
- [Rules](docs/rules.md) — rules by category
- [Integrations](docs/integrations.md) — IDE, CI, and MCP setup

## Contributing

```bash
git clone https://github.com/kaeawc/krit.git && cd krit
go build -o krit ./cmd/krit/
go test ./... -count=1
```

Rules live in `internal/rules/`. Fixtures live in `tests/fixtures/`. New rules
embed the appropriate rule base and register v2 metadata. See
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

See [LICENSE](LICENSE).
