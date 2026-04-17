# Build-integration cluster

No dedicated parent overview doc — the concepts here came from
comparing krit's Kotlin analysis capabilities (`scanner`,
`typeinfer`, `oracle`, `deadcode`) against the surface area a build
tool like [`grit`](https://github.com/kaeawc/grit) already covers
(Gradle/Android project model, action graph, cache keys, impact
analysis). The theme is exposing krit's symbol-level knowledge to
external build tooling so coarse file-hash and module-graph
heuristics can be replaced with precise, symbol-aware signals.

Every concept here is **tool-mode**, not a rule in the traditional
sense. They ship as subcommands, exported libraries, or long-lived
services. None of them emit findings.

## Cache-key precision

- [`abi-hash.md`](abi-hash.md) — deterministic public-API hash for
  compile-action cache keys. Sibling to
  [`architecture/public-api-surface-snapshot.md`](../architecture/public-api-surface-snapshot.md):
  same upstream walk, different consumer (machine hash vs. human
  diff).
- [`used-symbol-extraction.md`](used-symbol-extraction.md) —
  per-compilation-unit set of classpath symbols actually referenced,
  so downstream invalidation only fires when a *used* symbol changes.

## Reachability at project scale

- [`cross-module-dead-code.md`](cross-module-dead-code.md) — extend
  `internal/deadcode/` from per-file to a whole-project reachability
  pass that consumes a module edge list.
- [`symbol-impact-api.md`](symbol-impact-api.md) — given a changed
  symbol, return the transitive set of dependent files. Module-level
  impact analysis (which grit already does) pushed down to function
  and class granularity.

## Delivery mechanism

- [`analysis-daemon.md`](analysis-daemon.md) — long-lived
  scanner/oracle/typeinfer query server with incremental
  invalidation. Gates the others from being usable in a real build
  without re-parsing the world on every invocation.
