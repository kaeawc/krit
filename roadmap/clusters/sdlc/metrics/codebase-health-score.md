# CodebaseHealthScore

**Cluster:** [sdlc/metrics](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit score` — weighted sum of finding counts by severity, exposed
as JSON and a CI-friendly number.

## Shape

```
$ krit score --format json
{
  "score": 847,
  "grade": "B",
  "findingsBySeverity": {"error": 3, "warning": 120, "info": 280},
  "weights": {"error": 100, "warning": 10, "info": 1},
  "summary": {"total": 403, "files": 128, "rules": 472}
}
```

For CI, `krit score --format number` should print only the numeric
score so shell pipelines can compare it against a threshold without
JSON parsing.

## Dispatch

This should reuse the existing scan path, then swap the final output
formatter:

- file discovery and parsing from
  `internal/scanner/scanner.go` via `CollectKotlinFiles()` and
  `ScanFiles()`
- rule execution from `internal/rules/dispatch.go` via
  `NewDispatcher()` and `(*Dispatcher).Run()`
- optional baseline suppression from
  `internal/scanner/baseline.go` via `FilterByBaseline()`

In `cmd/krit/main.go`, the new subcommand should branch after the
normal finding collection/filtering path and before the existing
`output.FormatJSON()` / `output.FormatPlain()` switch. The scoring
step itself is a pure post-pass over `[]scanner.Finding`; it does not
need any new AST walk or rule hook.

## Infra reuse

- `internal/scanner/scanner.go` already carries the exact inputs the
  scorer needs on each finding: `Rule`, `RuleSet`, and `Severity`.
- `internal/output/json.go` already builds the stable JSON report
  shape in `FormatJSON()` and `JSONSummary`; `krit score` should
  mirror that summary style rather than inventing a second naming
  scheme.
- `internal/mcp/tools.go` (`severityLevel()`) and
  `internal/lsp/diagnostics.go` (`mapSeverity()`) already define the
  canonical severity ordering (`error` > `warning` > `info`). The new
  score weights should build on that same severity vocabulary instead
  of introducing rule-specific weight tables.
- The likely extraction point is a new shared scorer helper under
  `internal/output/` that accepts `[]scanner.Finding` and returns
  score totals plus the severity histogram, so both the CLI subcommand
  and JSON-oriented surfaces can reuse it.

## Links

- Parent: [`../README.md`](../README.md)
