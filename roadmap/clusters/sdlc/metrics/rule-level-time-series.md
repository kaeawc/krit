# RuleLevelTimeSeries

**Cluster:** [sdlc/metrics](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Per-commit finding counts saved to disk, queryable via the MCP
server. Lets teams say "we cut `LongMethod` by 60% this quarter".

## Shape

```
$ krit metrics log --out .krit/metrics.jsonl
$ krit metrics query LongMethod --since 2024-01-01
2024-01-15: 412
2024-04-01: 164 (-248)
```

`metrics log` should append one JSON object per run, not rewrite a snapshot file.
The cheapest first shape is to persist data Krit already computes today:

```json
{
  "timestamp": "2026-04-12T20:14:03Z",
  "version": "dev",
  "targets": ["."],
  "summary": {
    "total": 731,
    "byRule": {
      "LongMethod": 412,
      "MagicNumber": 183
    }
  },
  "perfTiming": [
    {"name": "collectFiles", "durationMs": 18},
    {"name": "parse", "durationMs": 94}
  ]
}
```

`metrics query` can then stay read-only: load the JSONL file, filter rows by
date + rule name, and print `(timestamp, count, delta)` tuples. The MCP surface
should return the same rows as structured JSON rather than inventing a second
schema.

## Dispatch

- CLI dispatch should follow the existing verb split in `cmd/krit/main.go`,
  where `baseline-audit` is detected before `flag.Parse()`. A future
  `metrics` verb can short-circuit the normal scan path the same way instead of
  threading more global flags through the default analyzer flow.
- `metrics log` should reuse the current analyzer pipeline rather than building
  a second scan stack: file discovery from
  `internal/scanner/scanner.go` (`CollectKotlinFiles`, `ScanFiles`),
  rule execution from `internal/rules/dispatch.go`
  (`NewDispatcher`, `(*Dispatcher).Run` / `RunWithStats`), and timing from
  `internal/perf/perf.go` (`New`, `Tracker.GetTimings`).
- `metrics query` is an offline file reader. It does not need tree-sitter or
  rule dispatch; it only needs the persisted JSONL schema plus date parsing and
  per-rule aggregation.
- MCP exposure should mirror the existing `analyze_project` wiring:
  add a new tool entry in `internal/mcp/tools.go` via `toolDefinitions()` and
  implement the request branch in `internal/mcp/server.go`
  `(*Server).handleToolsCall`.

## Infra reuse

- The current JSON formatter already computes the exact rollup this feature
  needs in `internal/output/json.go`: `FormatJSON()` fills
  `JSONSummary.ByRule`, `JSONSummary.Total`, and `PerfTiming`. The subcommand
  should reuse or extract that summarization logic instead of re-counting rules
  in a divergent code path.
- `internal/scanner/scanner.go` already defines the canonical finding payload
  (`scanner.Finding`), so the metrics log should be derived from that type's
  `Rule`, `Severity`, and file/location data rather than inventing a parallel
  finding model.
- `internal/mcp/tools.go` already has a close precedent in
  `(*Server).toolAnalyzeProject()`: it scans paths, runs the dispatcher, and
  returns a summary with top-rule counts. The time-series query tool can reuse
  its response-shaping style while swapping the backing data source from a live
  scan to the JSONL history file.
- `internal/mcp/prompts.go` should be able to consume the same MCP query tool
  later for prompts like "show rule trend before/after this refactor" without
  adding another transport-specific formatter.

## Links

- Parent: [`../README.md`](../README.md)
