# FlatFieldNames (postponed)

**Status:** postponed — blocked on upstream grammar.
**Originally drafted for:** `roadmap/clusters/flat-tree-migration/`.
**Moved here:** 2026-04-13.

## Why this is postponed

This concept proposed adding tree-sitter **field-name** lookup to
the flat tree, so rules could ask "give me the child tagged
`left` of this `equality_expression`" instead of counting
children positionally. It's the one piece of tree-sitter's API
that `FlatNode` + existing helpers don't cover.

Two facts make it a non-starter today:

1. **No current krit rule uses field names.** All 285+ rules
   find children structurally by node type (`simple_identifier`,
   `function_body`, `modifiers`, …) or by walking siblings. There
   is zero demand from existing rule code; `grep -rn
   'ChildByFieldName\|FieldNameForChild' internal/` returns
   nothing.

2. **The Kotlin grammar krit consumes provides no field
   metadata.** Krit pulls its Kotlin tree-sitter grammar from
   `github.com/smacker/go-tree-sitter/kotlin` (pinned in
   `go.mod`). That grammar's generated `parser.c` hard-codes
   `#define FIELD_COUNT 0` and wires it into the
   `TSLanguage.field_count` slot — meaning no production in the
   grammar was authored with tree-sitter's `field(...)` DSL
   helper, and `node.FieldNameForChild(i)` will *always* return
   `""` regardless of the node. Confirmed empirically: parsing a
   realistic Kotlin snippet (class, primary constructor,
   function with return type, `if` expression, `==` comparison)
   visits 51 children and 0 of them carry a field name.

So even if we built the `FlatChildByField` / `FlatHasField` /
`FlatFieldNameForChild` helpers and the `FlatTree.FieldIDs`
infrastructure cleanly, every lookup would return empty against
today's grammar. There's no rule that wants it and no grammar
that would answer it.

## What the rules would look like if we had it

Keeping the motivation on file so the next revisit has it. The
rule family that benefits from field names is anything that
needs to distinguish two children of the same grammatical type
by their *role*:

- `ConstantOnLeftInComparison` — flag `5 == user.role` Yoda
  conditions. `equality_expression.left` and `right` both parse
  to the same expression node type; positional lookup is the
  only way to tell them apart without field names.
- `IfBranchExpressionsIdentical` — compare
  `if_expression.consequence` vs `if_expression.alternative` to
  detect `if (x) foo() else foo()`. Both branches are
  `control_structure_body`.
- `ForLoopVariableNeverUsed` — `for_statement.iterator` (the
  loop variable) vs `for_statement.range` (what's iterated).
  Both are expression-ish.
- `NullableReturnOnNonNullFunction` —
  `function_declaration.type` (return type) separate from
  `parameters` and `body`.

All of these can be written today with positional plumbing +
operator-token splitting, just less cleanly and more fragile
against grammar updates.

## Paths to revive

Three approaches, roughly ordered by cost:

### 1. Accept the status quo (zero cost)

Rules continue to find children by type or position. "Brittle
against grammar updates" stays a latent concern, but the
practical blast radius is small because krit pins a specific
smacker/go-tree-sitter commit — grammar updates are explicit,
not drive-by. This is the path we're taking today.

### 2. Swap grammars (medium cost, uncertain payoff)

Find an alternative tree-sitter-kotlin grammar whose `parser.c`
has a non-zero `FIELD_COUNT`, and switch krit's scanner to use
it. Candidates to investigate: `fwcd/tree-sitter-kotlin` (what
smacker currently vendors — fields unlikely, needs
confirmation), `nexus-uw/tree-sitter-kotlin-ng`, or any fork
with active `field(...)` annotations in `grammar.js`.

Real work involved:
- Audit each candidate's `grammar.js` for `field(...)` calls,
  or check their generated `parser.c` for `FIELD_COUNT > 0`.
- Verify the candidate covers the Kotlin surface krit cares
  about (Compose, KSP, kapt annotations, string templates,
  multiline lambdas, context receivers, etc.) — regressions in
  grammar coverage are far worse than missing field names.
- Run the full rule suite + fixture tests against the new
  grammar and hunt silent diffs. The existing fixtures in
  `internal/rules/*_test.go` are the enforcement mechanism;
  expect a multi-day triage pass.
- Replace the `github.com/smacker/go-tree-sitter/kotlin` import
  with the new grammar's binding, or vendor it under
  `internal/scanner/kotlin/` the way `internal/tsxml/` vendors
  tree-sitter-xml.

Medium cost, but the payoff is uncertain: no Kotlin grammar in
the ecosystem is known a priori to have comprehensive field
coverage for the rule families listed above.

### 3. Fork the grammar and add fields ourselves (large cost)

Fork smacker's Kotlin grammar (or the underlying
`fwcd/tree-sitter-kotlin`), edit `grammar.js` to wrap the
relevant children in `field("left", ...)`, `field("right", ...)`,
`field("consequence", ...)`, etc., regenerate `parser.c` via
`tree-sitter generate`, and vendor the result under
`internal/scanner/kotlin/` the way `internal/tsxml/` vendors
tree-sitter-xml.

Real work involved:
- Stand up a JavaScript toolchain for `tree-sitter generate`.
- Learn enough of the Kotlin grammar's 143-symbol structure to
  place `field(...)` annotations correctly without breaking
  parse behavior. Grammar changes can silently shift the parse
  tree; every rule fixture becomes a regression test.
- Commit to long-term maintenance: every upstream grammar
  update has to be rebased onto our fork, and our
  `field(...)` annotations have to be reapplied.
- Build whatever CI is needed to regenerate `parser.c`
  reproducibly.

This is effectively "krit maintains its own Kotlin grammar," a
meaningful scope expansion to the project. Worth it only if
field-name lookup is one of several grammar gaps we want to fix,
not for this one concern in isolation.

## Decision

**Not justified today.** Path 1 is free, path 2 is speculative
work with an uncertain payoff, path 3 is a multi-week commitment
to a domain (JS grammar authoring + tree-sitter codegen) that
krit isn't otherwise in. No existing rule is blocked on this, so
the ROI is theoretical.

Revisit if any of these change:

- A rule request lands that *cannot* be cleanly expressed via
  type-based lookups — specifically one where positional child
  counting produces unmaintainable code reviewers push back on.
- A tree-sitter-kotlin grammar with non-zero `FIELD_COUNT`
  becomes available with equal or better Kotlin coverage than
  the current smacker vendor.
- krit decides to fork the Kotlin grammar for *other* reasons
  (grammar gaps, performance, Compose DSL support). If that
  happens, adding `field(...)` annotations is a cheap rider on
  the larger effort.

## Reference: implementation shape

Kept on file so a future revival doesn't have to re-derive it.
This is what the minimal `flat.go` addition would look like,
and it's cheap enough (~115 lines) that it can be resurrected
in a day once the grammar question is resolved.

```go
// FlatTree holds a preorder-flattened syntax tree.
type FlatTree struct {
    Nodes    []FlatNode
    // FieldIDs is a parallel slice indexed by node index. When
    // non-nil, FieldIDs[i] holds the interned grammar field-name
    // ID for node i as a child of its parent, or 0 if the child
    // carries no field name. FieldIDs is nil when no node in the
    // tree has a field tag, so field-free files pay zero
    // allocation.
    FieldIDs []uint16
}
```

- `flattenTree` calls `node.FieldNameForChild(i)` during its
  existing child walk. Non-empty results are interned via
  `internFieldName` into a package-level `FieldNameTable` (same
  pattern as `NodeTypeTable`) and written to `FieldIDs[childIdx]`.
  If no child in the tree carries a field name, `FieldIDs` stays
  nil and the result allocates nothing extra.
- Three package-level helpers on the existing style:
  `FlatChildByField(tree, parent, field) uint32`,
  `FlatFieldNameForChild(tree, child) string`,
  `FlatHasField(tree, parent, field) bool`. All bail early when
  `tree.FieldIDs == nil` so they're safe to call against any
  tree.
- Field-name interning reserves ID 0 as the "no field" sentinel,
  mirroring how `NodeTypeTable` would — but more strictly: ID 0
  is never assigned to a real name.
- Test coverage would cross-validate `FlatFieldNameForChild(idx)`
  against `sitter.Node.FieldNameForChild(i)` for every child of
  every node in a parsed Kotlin fixture.

A functional branch with this exact change applied (uncommitted,
~115 line diff on `internal/scanner/flat.go`) existed briefly in
`.claude/worktrees/flat-field-names` during the drafting of this
document. It was discarded once the upstream grammar blocker was
confirmed; recreating it is straightforward.

## Links

- Originating cluster: [`roadmap/clusters/flat-tree-migration/`](../clusters/flat-tree-migration/)
- Cluster parent: [`roadmap/68-flat-tree-migration.md`](../68-flat-tree-migration.md)
- Upstream grammar source: `github.com/smacker/go-tree-sitter/kotlin`
