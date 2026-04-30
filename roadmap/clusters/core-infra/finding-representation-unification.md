# FindingRepresentationUnification

**Cluster:** [core-infra](README.md) · **Status:** planned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Commits fully to `FindingColumns` as the only internal finding
representation. Deletes the `[]Finding` struct path and the four
nearly-identical deduplication functions in the fixer that exist
because both paths must be supported simultaneously.

Note: `FindingColumns` itself is already shipped
([`performance-infra/columnar-finding-storage.md`](../performance-infra/columnar-finding-storage.md)).
This item is about deleting the old path. The incremental item added
columnar storage alongside `[]Finding`; this item removes the remaining
parallel representation.

## Current cost

The codebase maintains two parallel representations:

| Representation | Where produced | Where consumed |
|---|---|---|
| `[]Finding` | Rule `CheckNode()` / `CheckLines()` return values | Most output paths, fixer `ApplyFixes()` |
| `FindingColumns` | Dispatcher collector | Fixer `ApplyAllFixesColumns()`, some output paths |

The fixer (`internal/fixer/fixer.go`) has four deduplication
functions that are nearly identical — two for each representation ×
two for byte-mode vs line-mode fixes:
- `deduplicateByteFixesReverse()`
- `deduplicateLineFixesReverse()`
- `deduplicateByteTextFixRowsReverse()`
- `deduplicateLineTextFixRowsReverse()`

If a file has both byte-mode and line-mode fixes, line-mode fixes are
silently dropped (fixer.go lines 124–128) because the two modes
cannot be mixed. This is a hidden failure mode with no user-facing
diagnostic.

Output formatters reconstruct per-finding structs from columnar data
at serialisation time, performing the conversion in both directions.

Relevant files:
- `internal/fixer/fixer.go` — four dedup functions, mixed-mode drop
- `internal/scanner/findings.go` — both `Finding` and `FindingColumns`
- `internal/output/` — dual-path formatters

## Proposed design

`CheckNode()` / `CheckLines()` stop returning `[]Finding`. Instead,
rules append directly to a `*FindingCollector` passed through
`Context`:

```go
// Rule check function signature
func(ctx *rules.Context) {
    if someCondition {
        ctx.Findings.Add(scanner.Finding{
            Line:    ctx.Node.StartLine,
            Col:     ctx.Node.StartCol,
            Message: "...",
        })
    }
}
```

The `FindingCollector` writes directly to `FindingColumns` — no
intermediate `[]Finding` allocation. The two fixer paths collapse to
one. With generics, the two deduplication variants (byte vs line) can
share one generic implementation.

## Migration path

1. Add `Findings *FindingCollector` to `rules.Context`
   (from [`unified-rule-interface.md`](unified-rule-interface.md)).
2. Write a shim: wrap existing rules so their `[]Finding` return
   value is forwarded to the collector. This keeps all rules working
   without modification during migration.
3. Migrate rule check functions to append directly to `ctx.Findings`
   in batches.
4. Once all rules are migrated, delete the `[]Finding` return type
   from the rule interface.
5. Delete `ApplyFixes()` (struct path); keep only
   `ApplyAllFixesColumns()`.
6. Collapse the four dedup functions to two generic variants:
   `deduplicateFixesReverse[T ByteFix | LineFix]()`.
7. Reject (or handle) mixed-mode fixes with a clear diagnostic rather
   than silently dropping line-mode fixes.

## Acceptance criteria

- `scanner.Finding` struct deleted or reduced to a serialisation
  facade with no internal use.
- `ApplyFixes()` and `ApplyFixesDetailed()` deleted from
  `internal/fixer/fixer.go`.
- Four dedup functions collapsed to ≤ two.
- Mixed-mode fix situation returns a descriptive error rather than
  silently dropping fixes.
- All fixer integration tests pass.

## Links

- Depends on: [`unified-rule-interface.md`](unified-rule-interface.md)
  (Context-based collector)
- See also: [`../performance-infra/columnar-finding-storage.md`](../performance-infra/columnar-finding-storage.md)
  (ships the columnar format; this item removes the old format)
- Related: `internal/fixer/fixer.go`, `internal/scanner/findings.go`
