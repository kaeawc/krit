# PerModuleScorecards

**Cluster:** [sdlc/metrics](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit scorecard` — per module: finding density, complexity
distribution, test ratio.

## Shape

```
$ krit scorecard --format markdown
| Module | Findings/1kLOC | Avg Complexity | Test Ratio |
|--------|----------------|---------------|------------|
| :core  | 2.3            | 4.1           | 1.2        |
| :ui    | 8.7            | 2.8           | 0.6        |
```

- CLI entrypoint should mirror the existing `baseline-audit` short-circuit in
  `cmd/krit/main.go`: detect `os.Args[1] == "scorecard"`, strip the verb, and
  dispatch into a dedicated helper such as `runScorecard(...)` in a new
  `cmd/krit/scorecard.go`.
- The subcommand should reuse the normal scan setup before reporting:
  `scanner.CollectKotlinFiles(...)` and `scanner.ScanFiles(...)` from
  `internal/scanner/scanner.go`, then the existing rule dispatcher path in
  `cmd/krit/main.go` that accumulates `allFindings` via
  `dispatcher.RunWithStats(file)`.
- Output should start with a markdown table formatter only, similar in spirit
  to the human-readable emitters in `cmd/krit/rule_audit.go`, with JSON added
  only after the metric shape stabilizes.

## Dispatch

- Module discovery and bucketing should follow the existing module-aware phase
  in `cmd/krit/main.go`:
  `module.DiscoverModules(scanRoot)`,
  `module.ParseAllDependencies(graph)`, and
  `module.BuildPerModuleIndexWithGlobal(graph, parsedFiles, moduleWorkers, codeIndex)`.
- Per-row module assignment should come from `(*module.ModuleGraph).FileToModule`
  in `internal/module/graph.go`, using the already-populated
  `PerModuleIndex.ModuleFiles` map from `internal/module/permodule.go` as the
  primary iteration surface.
- Findings/1kLOC should be computed by grouping `scanner.Finding` values from
  `internal/scanner/scanner.go` by module path, counting only files assigned to
  that module, then dividing by Kotlin LOC derived from `len(file.Lines)` on the
  corresponding `scanner.File`.
- Avg Complexity should be derived from the same AST traversal logic already used
  by complexity rules in `internal/rules/complexity.go`; the concrete helpers are
  `getComplexityMetrics(...)` / `collectComplexityMetrics(...)`. The cheapest
  implementation path is to factor that logic into an exported helper instead of
  reimplementing complexity scoring inside the subcommand.
- Test Ratio should be computed from module-relative source roots discovered by
  `findSourceRoots(...)` in `internal/module/discover.go`, with test LOC counted
  from roots under `src/test`, `src/androidTest`, `src/commonTest`, and
  `src/testFixtures`, divided by main-source LOC for the same module.

## Infra reuse

- Finding pipeline: `scanner.CollectKotlinFiles`, `scanner.ScanFiles`,
  `scanner.BuildIndex`, and the `dispatcher.RunWithStats(...)` path in
  `cmd/krit/main.go` already produce the per-file ASTs and `[]scanner.Finding`
  needed for density scoring.
- Module graph: `internal/module/discover.go` builds `Module.SourceRoots`,
  `internal/module/depparse.go` resolves inter-module dependencies, and
  `internal/module/permodule.go` already materializes `PerModuleIndex.ModuleFiles`
  for per-module aggregation.
- Complexity/cognitive logic: `internal/rules/complexity.go` already computes
  cyclomatic/cognitive metrics for function-like nodes; scorecard should reuse
  that traversal so the numbers stay aligned with `LongMethod`,
  `CyclomaticComplexMethod`, and `CognitiveComplexMethod`.
- Reporting precedent: `cmd/krit/rule_audit.go` and `cmd/krit/experiment_matrix.go`
  already contain small, reviewable plain-text table emitters that can be copied
  into a dedicated `scorecard` formatter without disturbing the main report path.

## Links

- Parent: [`../README.md`](../README.md)
