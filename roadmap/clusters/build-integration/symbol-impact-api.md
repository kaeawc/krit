# SymbolImpactApi

**Cluster:** [build-integration](README.md) · **Status:** planned · **Severity:** n/a (tool mode)

## Catches

Module-level impact analysis that conservatively invalidates too
much. A build tool today, given a changed file, walks the module
dependency graph and flags every transitive dependent as
potentially-affected. Most of those dependents do not reference the
specific symbol that changed. `krit impact` answers a finer
question: "given this *symbol* changed, which files transitively
depend on it?" — and returns the minimal set. Consumers use it to
prune CI test selection, compile-action scheduling, and code-review
impact summaries.

## Shape

```
$ krit impact com.acme.core.UserRepository.load
com.acme.feature-profile/src/main/kotlin/ProfileViewModel.kt
com.acme.feature-profile/src/main/kotlin/ProfileRepository.kt
com.acme.feature-auth/src/main/kotlin/AuthFlow.kt
  (3 files across 2 modules)

$ krit impact --json --from-file app/src/main/kotlin/Foo.kt
{
  "changed": ["com.acme.Foo.bar", "com.acme.Foo.Baz"],
  "impact": [
    {"file": "...", "symbols": ["com.acme.Foo.bar"]},
    ...
  ]
}
```

Two input modes:

- **Direct**: `krit impact <fqn> [<fqn>...]` — given symbols, list
  files that reference them (transitively).
- **Diff**: `krit impact --from-file <path>` or
  `krit impact --since <git-ref>` — krit computes the changed
  symbol set by diffing ABI (see [`abi-hash.md`](abi-hash.md)),
  then runs impact on that set.

## Dispatch

Build an inverted symbol index: for every declaration FQN,
precompute the set of files that reference it (via the same walk
as [`used-symbol-extraction.md`](used-symbol-extraction.md),
inverted). Look up input FQNs in the index, union the hitting file
sets.

For transitive impact, iterate: if a hit file's own public API
changes as a result (any of its reported used-symbols is in the
input set *and* the reference is exposed in the file's own ABI),
add the hit file's own symbols to the input set and repeat. Fixed
point terminates because the project is finite.

For the diff mode, the "changed symbol set" is the symmetric
difference between the ABI snapshot before and after — reuse the
walker from [`abi-hash.md`](abi-hash.md) and compare declaration
records, not just the final hash.

## Infra Reuse

- Reference collection / resolution: oracle +
  [`internal/typeinfer/api.go`](/Users/jason/kaeawc/krit/internal/typeinfer/api.go),
  shared with
  [`used-symbol-extraction.md`](used-symbol-extraction.md).
- Cross-file index: `scanner.BuildIndex(...)` in
  [`internal/scanner/index.go`](/Users/jason/kaeawc/krit/internal/scanner/index.go)
  — already holds declaration FQNs; needs an inverted-reference
  sidecar.
- Module graph: `internal/module/graph.go` for grouping output by
  module.
- Persistent cache of the inverted index across invocations:
  [`internal/cache/cache.go`](/Users/jason/kaeawc/krit/internal/cache/cache.go).
  Key the cache by per-file content hash; invalidate only the
  entries whose source file changed.
- This feature is the main reason
  [`analysis-daemon.md`](analysis-daemon.md) exists — rebuilding
  the inverted index from cold start on every invocation is the
  dominant cost and erases the savings of precise impact.

## Open questions

- **Overload dispatch.** `com.acme.Foo.bar` is ambiguous when `bar`
  is overloaded. Either require an arity or signature discriminator
  in the FQN, or treat the name as a wildcard and return the union.
  Lean toward the discriminator for cache correctness.
- **Virtual dispatch.** A call through an `interface` method can
  resolve to any implementer. The inverted index needs to record
  "called via interface `I`" so a change to any implementer of `I`
  also hits callers of `I.foo`. Reuse whatever oracle already does
  for the `CrossModuleScopeConsistency` DI check.
- **Reflection.** Same answer as the rest of this cluster: an
  explicit allow-list file, no automatic inference.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`abi-hash.md`](abi-hash.md), [`used-symbol-extraction.md`](used-symbol-extraction.md), [`analysis-daemon.md`](analysis-daemon.md)
