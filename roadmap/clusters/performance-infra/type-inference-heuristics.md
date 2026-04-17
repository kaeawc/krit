# Type Inference Heuristics Expansion

**Cluster:** [performance-infra](./README.md) · **Status:** shipped ·
**Supersedes:** [`roadmap/18-type-inference-heuristics.md`](../../18-type-inference-heuristics.md)

## What it is

Expand `internal/typeinfer/` with six heuristic areas to close detekt
type-resolution coverage gaps without requiring a Kotlin compiler or classpath:

1. **Stdlib signatures** — lookup tables for stdlib return types
2. **Exception hierarchy** — hard-coded Kotlin/Java exception subtype tree
3. **Source function return types** — infer from explicit return-type annotations
4. **Property propagation** — propagate types through val/var assignments
5. **Smart cast tracking** — follow is-checks through when/if branches
6. **Java interop** — resolve common Java stdlib types (Map, List, Set)

## Why

Moves krit from ~43% to ~64% coverage of detekt's type-resolution rules
(~20 rules unlocked) with zero external dependencies.

## Implementation notes

- Primary files: `internal/typeinfer/impl.go` (inferCallExpressionType),
  `types.go` (type maps), `resolver.go` (TypeResolver interface)
- Each expansion is independent and can land incrementally
- Related: item 10 (detekt coverage gaps), item 20 (kotlin type oracle)
