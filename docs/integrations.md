# Integrations

Krit's shipped integration today is the **raw CLI**, which emits
SARIF / Checkstyle / JSON / plain output for any CI runner or
pre-commit hook.

Editor (LSP) and AI-agent (MCP) integrations also ship as separate
binaries (`krit-lsp`, `krit-mcp`) installed alongside `krit`. For
configuring `krit-mcp` with Claude Code, Claude Desktop, Codex, or
any MCP-compatible client see [MCP setup](mcp.md).

## Pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/kaeawc/krit
    rev: <pinned tag>
    hooks:
      - id: krit          # analyze source files
      - id: krit-fix      # auto-fix source files (optional)
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

## Suggested fixes (CLI and IDE)

Some rules emit ordered, user-selectable **suggested fixes** instead
of a single autofix (see [Suggested fixes](suggested-fixes.md)). The
two surfaces below are how integrations apply one.

### CLI: `krit apply-suggestion`

```bash
krit --format=json . > findings.json

# List all findings that carry suggestions and their suggestion ids:
krit apply-suggestion --list findings.json

# Preview what a specific suggestion would change:
krit apply-suggestion \
  --finding 'PreferVal:src/main/kotlin/Example.kt:5:3' \
  --suggestion use-val \
  --dry-run \
  findings.json

# Apply it:
krit apply-suggestion \
  --finding 'PreferVal:src/main/kotlin/Example.kt:5:3' \
  --suggestion use-val \
  findings.json
```

The verb reuses the autofix application path
(`fixer.ApplyAllFixesColumns`), so cross-file edits, byte/line spans,
overlap deduplication, and ktfmt-shaped output match `krit --fix`.
Informational suggestions (no `edits`, only `applicationToken`) are
rejected — they are for the integration to interpret, not for the CLI
to write.

### IDE / LSP

Integrations should render each `suggestedFixes[]` entry as its own
native quick-fix / code-action, **in the order the rule emitted them**
— do not sort by id, title, or safety level. The IntelliJ plugin
follows this contract:

- `KritInspection` registers one `KritApplySuggestionQuickFix` per
  machine-applicable suggestion (those with non-empty `edits`).
- When suggestions are present, the generic "apply autofix" entry is
  suppressed so the user is never offered both choices at once.
- Informational suggestions (no `edits`) are surfaced as intentions
  that route through the integration's help/`applicationToken`
  handling.

For LSP wrappers, expose each suggestion as a distinct `CodeAction`
with a stable `title` (the rule-provided `Title`) and a `data` blob
that carries `{findingId, suggestionId}` so the resolve step can run
`krit apply-suggestion` with deterministic ids.
