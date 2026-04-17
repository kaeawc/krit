# Integrations

Krit's shipped integration today is the **raw CLI**, which emits
SARIF / Checkstyle / JSON / plain output for any CI runner or
pre-commit hook.

Editor (LSP) and AI-agent (MCP) integrations exist in the source
tree but are not yet validated end-to-end — track progress in
[`roadmap/clusters/lsp/`](https://github.com/kaeawc/krit/tree/main/roadmap/clusters/lsp)
and the MCP entry in
[`roadmap/clusters/release-engineering/distribution-readiness.md`](https://github.com/kaeawc/krit/blob/main/roadmap/clusters/release-engineering/distribution-readiness.md).

## Pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/kaeawc/krit
    rev: v0.1.0
    hooks:
      - id: krit          # analyze Kotlin files
      - id: krit-fix      # auto-fix Kotlin files (optional)
```

`krit` runs analysis and fails the commit on findings. `krit-fix` runs
`krit --fix` over the staged files — use one or the other, not both in
the same hook stage.

## Output formats for CI

| Format     | Use case                    | Flag |
|------------|-----------------------------|------|
| SARIF      | GitHub Code Scanning        | `--format sarif` |
| Checkstyle | Jenkins, other CI tools     | `--format checkstyle` |
| JSON       | Custom processing (default) | `--format json` |
| Plain      | Human-readable logs         | `--format plain` |
