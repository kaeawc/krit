**Cluster:** performance-core · **Est.:** 0.5 day · **Depends on:** none

## What

`collectReferencesFlat` (`internal/scanner/index.go:691-708`) runs an
unconditional `inComment` ancestor check on **every** AST node before
filtering to identifier types. Two `FlatHasAncestorOfType` calls per
node × ~3000 nodes per file × 3442 Kotlin files on Signal-Android adds
up to millions of parent-pointer walks whose result is discarded the
moment the subsequent type filter returns.

```go
func collectReferencesFlat(file *File, refs *[]Reference) {
    file.FlatWalkAllNodes(0, func(idx uint32) {
        nodeType := file.FlatType(idx)
        // ↓ runs for every node, including the ~80–90% that
        //   will be rejected by the filter below
        inComment := nodeType == "line_comment" ||
            nodeType == "multiline_comment" ||
            file.FlatHasAncestorOfType(idx, "line_comment") ||
            file.FlatHasAncestorOfType(idx, "multiline_comment")
        if nodeType != "simple_identifier" && nodeType != "type_identifier" {
            return
        }
        // ...
    })
}
```

Additionally, the two self-comparisons at the start (`nodeType ==
"line_comment"` / `"multiline_comment"`) are dead code: any node of
those types is rejected by the next-line type filter before `inComment`
is consumed. And the two `FlatHasAncestorOfType` calls walk the same
ancestor chain twice, once per type.

## Measurement

Cold-run benchmark on Signal-Android (`scratch/benchmark-runbook.md`,
M-class 16-core, commit `e1257dc`):

| Phase | Cold | Warm |
|---|---:|---:|
| kotlinIndexCollection | 1,049 ms | — (loaded from shard cache) |
| crossFileAnalysis (total) | 7,642 ms | 512 ms |

`collectReferencesFlat` is called inside `kotlinIndexCollection` on the
cold-run / missed-shard path. Back-of-envelope: avg Kotlin file has ~3k
nodes, ~1k identifiers; the redundant-walk cost is `(3k − 1k) × 2
ancestor walks × avg depth 6 × 3442 files ≈ 83M parent-pointer reads`.
At ~5 ns each that's ~400 ms of pure pointer-chasing on cold.

**Target:** cold `kotlinIndexCollection` drops by 200–400 ms. No impact
on warm (already served by the shard cache).

## Plan

1. Move the type filter above the `inComment` computation so only
   identifier nodes pay for an ancestor walk.
2. Fuse the two `FlatHasAncestorOfType` calls into a single ancestor
   walk that checks both `line_comment` and `multiline_comment` types
   in one pass — either via a new `FlatHasAnyAncestorOfType(ids
   ...uint16)` helper on `*File`, or inline loop using the already-
   resolved `typeID` lookups.
3. Drop the dead self-comparisons (`nodeType == "line_comment"` etc.).

No behavioural change: the `InComment` flag populated on each
`Reference` must be byte-identical to the current output.

## Expectations

- Cold `kotlinIndexCollection` drops 20–40%.
- Warm is unaffected (shard-cache hit, this code doesn't run).
- No change to findings — `--report json` diff is empty.
- Works correctly on Java refs path (`collectJavaReferencesFlat` at
  `index.go:733-752`) — that function doesn't currently compute
  `inComment`, so no regression there.

## Validate

- `go test ./internal/scanner/ -count=1` — existing index tests cover
  the `InComment` flag on references inside `//` and `/* */`.
- Add one test: reference inside a KDoc block has `InComment=true`
  after the change (regression guard for the ancestor-walk fusion).
- Signal-Android cold run: `kotlinIndexCollection` drops ≥200 ms.
- Finding-equivalence: `./krit --report json ~/github/Signal-Android`
  produces identical output pre vs post.

## Risks

- Ancestor-walk fusion — the single-pass version must match the
  semantics of two independent `FlatHasAncestorOfType` calls. The
  existing function (`flat.go:393-407`) already resolves `typeID` via
  `lookupNodeType`; reuse that.
- Low risk overall: the change is a mechanical reorder + helper
  introduction. No format / cache / schema churn.

## Opportunities

- If the fused helper lands as a first-class `File` method, other
  rules that do similar ancestor-type checks can adopt it and drop
  their ad-hoc loops.
- Pairs with the `pre-compute in-comment bitset during flattenTree`
  idea (cached-at-parse-time) — saved for a later issue if even the
  O(identifier-count) cost becomes a bottleneck.

## References

- Code: `internal/scanner/index.go:691-708`
- Helper: `internal/scanner/flat.go:393-407` (`FlatHasAncestorOfType`)
- Measurement: benchmark runs in
  `scratch/benchmarks/2026-04-21_043420_7a182cb_cold.json`
