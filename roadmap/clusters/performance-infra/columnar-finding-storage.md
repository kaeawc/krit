# ColumnarFindingStorage

**Cluster:** [performance-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Replace `[]Finding` (array of structs with string pointers) with
parallel scalar slices the GC skips entirely.

## Current cost

Each `scanner.Finding` has 7 string/pointer fields (`File`, `Rule`,
`RuleSet`, `Severity`, `Message`, `*Fix`, `*BinaryFix`). On a scan
producing 1000 findings, the GC traces 7000+ pointer fields every
cycle. The struct layout also has poor cache locality for
sort-by-file-and-line, which is the primary output operation.

## Proposed layout

```go
type FindingColumns struct {
    FileIdx    []uint32 // index into interned file path table
    Line       []uint32
    Col        []uint16
    RuleIdx    []uint16 // index into rule name table
    SeverityID []uint8  // 0=info, 1=warning, 2=error
    MessageIdx []uint32 // index into message intern table
    Confidence []uint8  // 0-100 (scaled from 0.0-1.0)
    FixStart   []uint32 // 0 = no fix; otherwise index into FixPool
    N          int
}
```

All slices are scalar — no pointers, no GC tracing. Sorting by
file+line is a parallel index permutation over flat arrays.

## Migration path

1. Add `FindingColumns` and a `FindingCollector` builder to
   `internal/scanner/findings.go`.
2. Rules continue to return `[]Finding` from `CheckNode` /
   `CheckLines` — the dispatcher converts to columnar on the fly
   via `collector.Append(finding)`.
3. Output formatters (`internal/output/`) read from `FindingColumns`
   and reconstitute per-finding structs only at serialization time.
4. The existing `[]Finding` return type can be preserved as a
   compatibility facade that reads from the columns.

## Expected impact

GC time for the finding pipeline drops to near-zero. Sorting 10k
findings by file+line becomes a cache-friendly integer sort instead
of a pointer-chasing struct sort. Modest win on scan time; large
win on memory pressure for repos with 10k+ findings.

## Acceptance criteria

- `go test -bench` comparing sort performance on 10k synthetic
  findings: columnar ≥ 2x faster than `[]Finding` sort.
- GC profile shows zero pointer tracing in the finding path.
- No rule-author-facing API change.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Depends on: [`string-interning.md`](string-interning.md) (message
  and file path interning feeds the index tables)
