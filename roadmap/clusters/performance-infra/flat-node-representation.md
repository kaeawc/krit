# FlatNodeRepresentation

**Cluster:** [performance-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

After `parser.ParseCtx` returns a tree-sitter `*sitter.Tree`, walk
the C tree exactly once via cgo and serialize every node into a
contiguous Go slice of value-type structs. All subsequent rule
dispatch, `FindChild`, `WalkNodes`, `HasModifier`, `NodeText`, etc.
operate on this flat slice with zero cgo.

## Current cost

Every `node.Child(i)`, `node.Type()`, `node.StartByte()`,
`node.EndByte()` is a cgo call. A 500-line Kotlin file has ~2000
nodes. With 472 rules each touching 5–10 fields per matching node,
a single file generates millions of cgo transitions across a full
scan.

## Proposed struct

```go
type FlatNode struct {
    Type       uint16 // index into NodeTypeTable
    Parent     uint32 // index into []FlatNode
    FirstChild uint32 // index, 0 = no children
    NextSib    uint32 // index, 0 = last sibling
    StartByte  uint32
    EndByte    uint32
    StartRow   uint16
    StartCol   uint16
    ChildCount uint16
    NamedCount uint16 // named child count (tree-sitter distinction)
    Flags      uint8  // named bit, error bit
    _pad       uint8
}
// 28 bytes per node, cache-line aligned for sequential scan
```

## NodeTypeTable

A small interned string table (~200 entries for Kotlin grammar):

```go
var NodeTypeTable []string // indexed by FlatNode.Type
var nodeTypeIndex map[string]uint16
```

`node.Type() == "call_expression"` becomes
`node.Type == nodeTypeIndex["call_expression"]` — integer comparison,
no string allocation, no cgo.

## Flattening walk

A single recursive cgo traversal in preorder:

```go
func flattenTree(root *sitter.Node, content []byte) []FlatNode {
    nodes := make([]FlatNode, 0, root.ChildCount()*4)
    var walk func(n *sitter.Node, parent uint32)
    walk = func(n *sitter.Node, parent uint32) {
        idx := uint32(len(nodes))
        nodes = append(nodes, FlatNode{
            Type:       internNodeType(n.Type()),
            Parent:     parent,
            StartByte:  uint32(n.StartByte()),
            EndByte:    uint32(n.EndByte()),
            StartRow:   uint16(n.StartPoint().Row),
            StartCol:   uint16(n.StartPoint().Column),
            ChildCount: uint16(n.ChildCount()),
            NamedCount: uint16(n.NamedChildCount()),
        })
        // Wire parent's FirstChild
        if parent < idx {
            p := &nodes[parent]
            if p.FirstChild == 0 && idx > 0 {
                p.FirstChild = idx
            }
        }
        // Wire previous sibling
        if idx > 0 && nodes[idx-1].Parent == parent {
            nodes[idx-1].NextSib = idx
        }
        for i := 0; i < int(n.ChildCount()); i++ {
            walk(n.Child(i), idx)
        }
    }
    walk(root, 0)
    return nodes
}
```

One cgo traversal, O(N) time, O(N) space. After this, `root` and
the `*sitter.Tree` can be released immediately — no further cgo.

## Migration path

1. Add `FlatNode`, `FlatTree`, `flattenTree` to
   `internal/scanner/flat.go`.
2. Add `FlatTree` field to `scanner.File` alongside the existing
   `Tree *sitter.Tree`.
3. Add compatibility wrappers: `FlatFindChild`, `FlatWalkNodes`,
   `FlatHasModifier`, `FlatNodeText` that operate on `[]FlatNode` +
   `[]byte` content.
4. Migrate rules one batch at a time — each rule switches from
   `scanner.FindChild(node, ...)` to `flat.FindChild(tree, idx, ...)`.
   No interface change needed; the dispatcher passes both `*File` and
   the flat tree index.
5. Once all rules are migrated, remove the `Tree *sitter.Tree` field
   and the cgo-based helpers. The parser pool stays (cgo is still
   needed for parsing), but everything after parsing is pure Go.

## Expected impact

3–5x improvement on the AST-walk phase, which is ~60% of total scan
time on large repos. On Signal-Android (~5500 files) this should
reduce the walk phase from ~2s to ~500ms.

## Acceptance criteria

- `krit --perf` on Signal-Android shows measurable improvement in
  the `dispatch` timing bucket.
- All existing tests pass without modification (the wrappers maintain
  the same semantics).
- `go test -bench` on a synthetic 10k-node tree shows the flat walk
  is ≥3x faster than the cgo walk.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Infra home: `internal/scanner/flat.go` (new)
- Consumers: every `DispatchRule.CheckNode` implementation
