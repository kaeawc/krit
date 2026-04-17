# ZeroCopyNodeText

**Cluster:** [performance-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Return `[]byte` slices into the original file content instead of
allocating a new `string` per `NodeText` call. Convert to `string`
only when needed for map lookups, finding messages, or return values.

## Current cost

`scanner.NodeText(node, content)` does
`string(content[start:end])` — a heap allocation per call. A rule
that checks multiple modifiers on one node (`HasModifier` checks
"abstract", "override", "open", etc.) allocates one string per
child per check.

## Proposed API

```go
// Zero-copy: returns a slice of the original content.
// Valid only while content is live.
func NodeBytes(nodes []FlatNode, idx uint32, content []byte) []byte {
    n := nodes[idx]
    return content[n.StartByte:n.EndByte]
}

// String conversion only when needed:
func NodeString(nodes []FlatNode, idx uint32, content []byte, pool *StringPool) string {
    b := NodeBytes(nodes, idx, content)
    return pool.Intern(string(b))
}
```

Go's compiler optimizes `string(bytes) == "abstract"` to avoid
allocation when the comparison target is a string constant. So
modifier checks become:

```go
// Before: allocates
text := scanner.NodeText(child, content)
if text == "abstract" { ... }

// After: zero-alloc comparison
b := flat.NodeBytes(nodes, childIdx, content)
if string(b) == "abstract" { ... }
```

The compiler sees `string(b)` used only in a comparison with a
constant and skips the heap allocation entirely.

## Where to apply

- `HasModifier` — the single hottest helper; called by ~100 rules
- `FindChild` type comparisons — already uint16 with flat nodes
- `extractIdentifier` — returns interned string, reads zero-copy
- Rule-specific text checks in `CheckNode` bodies

## Expected impact

Modest per-call savings (~50ns per avoided allocation), but called
millions of times across a full scan. The compound effect is
significant: `HasModifier` alone accounts for ~15% of allocations
in a profile of a 1000-file scan.

## Acceptance criteria

- `go test -bench -memprofile` shows ≥ 50% reduction in
  `NodeText`-attributed allocations.
- `HasModifier` benchmark shows zero allocations per call for the
  common case (modifier found via byte comparison).
- No API change for rules that receive strings — the interned path
  still returns `string`.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Depends on: [`flat-node-representation.md`](flat-node-representation.md),
  [`string-interning.md`](string-interning.md)
