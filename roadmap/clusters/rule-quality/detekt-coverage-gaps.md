# Detekt Type-Resolution Coverage Gaps

**Cluster:** [rule-quality](./README.md) · **Status:** in-progress ·
**Supersedes:** [`roadmap/10-detekt-coverage-gaps.md`](../../10-detekt-coverage-gaps.md)

## What it is

~19 detekt rules that require type resolution remain unimplemented or
operate at reduced accuracy. These are the gap between krit's 227/227
detekt core coverage (syntactic parity) and full behavioral parity.

## Current state

~75/94 type-resolution-dependent rules have working implementations.
~10 more are achievable with expanded type-inference heuristics
(see `performance-infra/type-inference-heuristics.md`). ~9 are hard-blocked
on full Kotlin compiler type resolution (oracle item 20).

## Remaining gaps (approximate)

Rules needing heuristic expansion: UselessCallOnNotNull,
UnnecessaryNotNullOperator, UnnecessarySafeCall, SafeCast,
UnsafeCast, and others in potentialbugs_types.go / potentialbugs_nullsafety.go.

Rules blocked on oracle: ImplicitDefaultLocale, InvalidRange,
EqualsWithHashCodeExist (needs full class hierarchy).

## Implementation notes

- Primary files: `internal/rules/potentialbugs_*.go`, `internal/typeinfer/`
- Each rule can land independently
- Related: item 18 (type inference heuristics), item 20 (oracle)
