# TypeResolutionService

**Cluster:** [core-infra](README.md) · **Status:** planned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Eliminates the manual `SetResolver()` setter injection pattern for
type-aware rules. The type resolver becomes a field on `rules.Context`,
populated by the dispatcher when the rule declares `NeedsResolver` in
its capabilities. The JVM oracle becomes a pluggable backend behind the
same `TypeResolver` interface with a defined precedence order
(oracle > source-level > skip).

## Current cost

Type-aware rules implement a separate `TypeAwareRule` interface that
requires a `SetResolver(typeinfer.TypeResolver)` method. The resolver
is injected by the dispatcher — but only after an explicit call in
`cmd/krit/main.go`:

```go
// cmd/krit/main.go line 1253 (approximate)
for _, r := range activeRules {
    if tr, ok := r.(rules.TypeAwareRule); ok {
        tr.SetResolver(resolver)
    }
}
```

This call does not exist in the LSP server or MCP server dispatcher
setup, so type-aware rules silently degrade (produce no type-based
findings) in those entry points. There is no error, no warning, no
indication to the user that type information is unavailable.

The dual backends (source-level `typeinfer.TypeResolver` and JVM
oracle) have no defined precedence. Rules that call both can get
conflicting answers with no tie-breaking logic.

Relevant files:
- `internal/rules/rule.go` — `TypeAwareRule` interface
- `cmd/krit/main.go:~1253` — manual resolver injection
- `internal/typeinfer/api.go` — `TypeResolver` interface
- `internal/oracle/` — JVM oracle client

## Proposed design

`TypeAwareRule` is deleted. Rules that need type information declare
`NeedsTypeInfo` in their `Capabilities` bitfield (from
[`unified-rule-interface.md`](unified-rule-interface.md)).
`NeedsTypeInfo` is the unified capability: it is defined as
`NeedsResolver | NeedsOracle` and asks the dispatcher to (a) populate
`ctx.Resolver` with a `ChainedResolver` whose backends are ordered
oracle > source-level, and (b) include the rule in the oracle's
file-selection pass. Rules authored against `NeedsTypeInfo` never need
to know which backend they will be served by at call time — the
resolver service picks the cheapest backend internally.

The individual `NeedsResolver` / `NeedsOracle` bits remain as the
lower-level primitives and continue to work for existing rules, but
new rules should prefer `NeedsTypeInfo`.

The dispatcher populates `ctx.Resolver` before calling the rule's
`Check` function.

```go
type Context struct {
    // ...
    // Non-nil only when rule declares NeedsResolver:
    Resolver typeinfer.TypeResolver
}
```

`TypeResolver` is backed by a `ChainedResolver` that tries backends
in priority order:

```go
type ChainedResolver struct {
    backends []TypeResolver // oracle first, source-level second
}

func (c *ChainedResolver) ResolveType(node *scanner.FlatNode, file *scanner.ParsedFile) (Type, bool) {
    for _, b := range c.backends {
        if t, ok := b.ResolveType(node, file); ok {
            return t, true
        }
    }
    return Type{}, false
}
```

If no backends are configured (e.g., in a quick scan without the
oracle), rules that declared `NeedsResolver` are skipped with a
`--verbose` log entry rather than silently returning no findings.

## Migration path

1. Add `Resolver typeinfer.TypeResolver` to `rules.Context`.
2. Add `NeedsResolver` capability bit to the `Capabilities` type.
3. Update the dispatcher to populate `ctx.Resolver` from the
   `ChainedResolver` for rules that declare `NeedsResolver`.
4. Migrate `TypeAwareRule` implementations: replace `SetResolver` with
   `NeedsResolver` capability; replace `r.resolver` field accesses
   with `ctx.Resolver`.
5. Delete `TypeAwareRule` interface and all `SetResolver()` methods.
6. Delete the manual resolver injection loop from `cmd/krit/main.go`,
   `lsp/server.go`, and `mcp/server.go`.
7. Add the resolver to the LSP and MCP dispatcher setup so type-aware
   rules work in those entry points.

## Acceptance criteria

- `TypeAwareRule` interface deleted.
- No `SetResolver()` calls anywhere in the codebase.
- Type-aware rules produce findings in the LSP server (verified by
  integration test).
- Rules that declare `NeedsResolver` when no resolver is available are
  skipped with a `--verbose` diagnostic, not silently no-op.
- Precedence: oracle findings take priority over source-level when
  both are present for the same node.

## Links

- Depends on: [`unified-rule-interface.md`](unified-rule-interface.md)
  (Capabilities bitfield, Context type)
- Depends on: [`phase-pipeline.md`](phase-pipeline.md) (resolver
  wired in pipeline rather than in each entry point)
- Related: `internal/typeinfer/api.go`, `internal/oracle/`,
  `internal/rules/rule.go`
