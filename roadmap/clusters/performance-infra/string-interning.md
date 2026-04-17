# StringInterning

**Cluster:** [performance-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Deduplicate repeated string values across all files during a scan so
the GC sees one live object per unique string instead of thousands of
identical copies.

## Current cost

`NodeText(node, content)` allocates a new `string` every call. On a
500-file scan, identifiers like `override`, `fun`, `val`, package
prefixes like `com.example`, and import paths like
`kotlinx.coroutines.flow` are allocated thousands of times. Each
is a separate heap object the GC must trace.

## Proposed API

```go
// internal/scanner/intern.go
type StringPool struct {
    mu    sync.Mutex
    table map[string]string
}

func (p *StringPool) Intern(s string) string {
    p.mu.Lock()
    if v, ok := p.table[s]; ok {
        p.mu.Unlock()
        return v
    }
    p.table[s] = s
    p.mu.Unlock()
    return s
}

// Per-worker unsynchronized variant for hot paths:
type LocalPool struct {
    table map[string]string
    fallback *StringPool
}

func (p *LocalPool) Intern(s string) string {
    if v, ok := p.table[s]; ok {
        return v
    }
    // Check global, promote to local
    v := p.fallback.Intern(s)
    p.table[v] = v
    return v
}
```

## Where to apply

- `NodeText` / `FlatNodeText` — intern the result before returning
- Node type strings — already handled by `NodeTypeTable` from the
  flat node representation; this covers identifiers
- `scanner.File.Path` — intern file paths
- `scanner.Finding.Rule`, `.RuleSet`, `.Severity` — small fixed set,
  intern trivially
- Import paths in the cross-file index
- Package names in `CodeIndex`

## Expected impact

20–30% reduction in total heap allocations. Measurable drop in GC
pause time on 1000+ file scans. The `alloc_objects` profile bucket
for `NodeText` should drop by ~80%.

## Acceptance criteria

- `go test -bench -memprofile` shows allocation reduction on a
  synthetic multi-file benchmark.
- No API change for rule authors — interning is internal to the
  scanner layer.
- Thread-safe: the global pool is mutex-protected; per-worker local
  pools are unsynchronized and merged at worker completion.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Depends on: [`flat-node-representation.md`](flat-node-representation.md)
  (the flat node's `Type` field uses the same interning concept)
- Infra home: `internal/scanner/intern.go` (new)
