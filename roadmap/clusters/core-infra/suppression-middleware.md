# SuppressionMiddleware

**Cluster:** [core-infra](README.md) · **Status:** planned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Merges the four current suppression mechanisms — `@Suppress`
annotations, baseline files (detekt XML format), config `excludes`
glob patterns, and `// krit:ignore` inline comments — into a single
`SuppressionFilter` that is built once per file and applied uniformly
by the dispatcher. Eliminates the per-file annotation index rebuild
that currently happens on every file in every scan.

## Current cost

Suppression is applied through four disconnected mechanisms:

1. **Annotation-based**: `BuildSuppressionIndexFlat()` is called for
   every file inside the dispatcher's per-file loop
   (`internal/rules/dispatch.go`). This rebuilds the index from
   scratch on each call — even for files that have not changed.

2. **Baseline**: loaded as a separate `BaselineSet` from a detekt XML
   file, applied after findings are collected, in a different code
   path than annotation suppression.

3. **Config excludes**: `SetRuleExcludes()` maps rule name → glob
   patterns; evaluated per-rule per-file during dispatch. No way to
   exclude a file from all rules at once.

4. **Inline `// krit:ignore` comments**: handled by a separate scanner
   pass that is not integrated with the other three mechanisms.

The result:
- Adding a new suppression source (e.g., `// noinspection` for IDE
  compatibility) requires edits in multiple disconnected locations.
- The per-file annotation index rebuild accounts for measurable
  overhead on large files with many annotations.
- Cross-file rules are not subject to annotation suppression at all
  (they bypass the dispatcher's per-file loop).

Relevant files:
- `internal/scanner/suppress.go` — `BuildSuppressionIndexFlat()`
- `internal/scanner/baseline.go` — `BaselineSet`
- `internal/rules/dispatch.go` — annotation index called per file
- `internal/rules/config.go` — `SetRuleExcludes()`

## Proposed design

A single `SuppressionFilter` is built once per file during the Parse
phase and cached on `ParsedFile`:

```go
// internal/scanner/suppress.go

type SuppressionFilter struct {
    // Combined view: rule name → set of line ranges suppressed
    byRuleAndLine map[string][]LineRange
    // Rule names suppressed for the whole file
    wholeFile map[string]bool
    // File is entirely excluded (all rules)
    excluded bool
}

func (f *SuppressionFilter) IsSuppressed(rule string, line int) bool

// BuildSuppressionFilter combines all suppression sources for one file.
// baseline and excludes are project-level; the rest come from the file.
func BuildSuppressionFilter(
    file *ParsedFile,
    baseline *BaselineSet,
    excludes map[string][]string, // rule → glob patterns
) *SuppressionFilter
```

The dispatcher calls `filter.IsSuppressed(rule.ID, finding.Line)`
instead of maintaining separate suppression checks. The filter is
built in the Parse phase and cached on `ParsedFile.Suppression`.

Cross-file rules receive the same filter and are suppression-checked
identically.

## Migration path

1. Define `SuppressionFilter` and `BuildSuppressionFilter()` in
   `internal/scanner/suppress.go`.
2. Add `Suppression *SuppressionFilter` field to `scanner.ParsedFile`.
3. Populate it in the Parse phase (from
   [`phase-pipeline.md`](phase-pipeline.md)).
4. Update the dispatcher to call `filter.IsSuppressed()` instead of
   consulting the annotation index, baseline, and exclude maps
   separately.
5. Remove the inline `BuildSuppressionIndexFlat()` call from the
   dispatcher's per-file loop.
6. Update cross-file rule execution to apply the same filter.
7. Delete `BuildSuppressionIndexFlat()` (or demote to internal use
   inside `BuildSuppressionFilter()`).

## Acceptance criteria

- `BuildSuppressionIndexFlat()` is no longer called inside the
  dispatcher's per-file loop.
- Annotation, baseline, config-exclude, and inline-comment suppression
  all go through `SuppressionFilter.IsSuppressed()`.
- Cross-file rules are subject to suppression (verified by new
  fixtures).
- `go test -bench` on the dispatcher shows reduced allocs per file on
  a corpus with dense `@Suppress` usage.
- Existing suppression fixtures all pass without modification.

## Links

- Depends on: [`phase-pipeline.md`](phase-pipeline.md) (filter built
  in Parse phase, cached on ParsedFile)
- Related: `internal/scanner/suppress.go`,
  `internal/scanner/baseline.go`, `internal/rules/dispatch.go`
