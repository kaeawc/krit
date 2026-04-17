# UsedSymbolExtraction

**Cluster:** [build-integration](README.md) · **Status:** planned · **Severity:** n/a (tool mode)

## Catches

Over-conservative reverse-dep invalidation. A build tool today
invalidates every downstream consumer of a library when *any*
classpath entry it depends on changes. Most of that work is
wasted: the consumer only actually referenced a small slice of
symbols from that library. `krit used-symbols` returns the
minimal set of external symbols a compilation unit references,
so a build tool can re-run the compile only when one of *those*
symbols changes.

## Shape

```
$ krit used-symbols :feature-profile
:feature-profile
  com.acme.core.UserRepository
  com.acme.core.UserRepository.load
  com.acme.core.UserRepository.Result
  kotlinx.coroutines.flow.Flow
  kotlinx.coroutines.flow.map

$ krit used-symbols --json app/src/main/kotlin/com/acme/Foo.kt
{"file":"app/src/main/kotlin/com/acme/Foo.kt","symbols":[
  {"fqn":"com.acme.core.UserRepository","kind":"class"},
  {"fqn":"com.acme.core.UserRepository.load","kind":"function","arity":1}
]}
```

Combined with [`abi-hash.md`](abi-hash.md), a build tool can key a
compile action on `hash(used-symbols ∩ upstream-ABI)` instead of
`hash(upstream-ABI)`. Editing a public function that nothing in the
downstream actually calls no longer triggers a downstream rebuild.

## Dispatch

Walk every `call_expression`, `navigation_expression`, `type_identifier`,
`user_type`, `delegation_specifier`, `annotation`, and
`import_header` in the compilation unit. For each, ask oracle to
resolve the target:

- `navigation_expression` / `call_expression` → resolved callable
  FQN, including receiver type for extension functions.
- `type_identifier` / `user_type` → resolved class FQN.
- `delegation_specifier` → supertype FQN (interfaces and classes
  both count).
- `annotation` → annotation class FQN, only if retention is
  `BINARY` or `RUNTIME`.
- `import_header` → treat as a coarse fallback when oracle cannot
  resolve a specific usage (e.g. in an unparseable subtree).

Filter out symbols whose FQN lives inside the same compilation unit
or the same module (internal references do not cross the cache-key
boundary we care about). The remaining set is the output.

## Infra Reuse

- Reference resolution: oracle's query entry points in
  [`internal/oracle/oracle.go`](/Users/jason/kaeawc/krit/internal/oracle/oracle.go)
  and
  [`internal/oracle/invoke.go`](/Users/jason/kaeawc/krit/internal/oracle/invoke.go).
- Type-aware lookups: typeinfer's public API in
  [`internal/typeinfer/api.go`](/Users/jason/kaeawc/krit/internal/typeinfer/api.go).
- Same-module filter: `(*ModuleGraph).FileToModule(...)` in
  [`internal/module/graph.go`](/Users/jason/kaeawc/krit/internal/module/graph.go).
- File enumeration: `scanner.CollectKotlinFiles(...)` in
  [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go).
- Cache of per-file used-symbol sets keyed by file content hash:
  [`internal/cache/cache.go`](/Users/jason/kaeawc/krit/internal/cache/cache.go).

## Open questions

- **Reflection and DI frameworks.** Hilt, Anvil, Kodein, and
  plain `Class.forName` look up symbols at runtime. Used-symbol
  extraction cannot see those. Either emit a "this module uses
  reflection" flag so consumers fall back to coarse invalidation,
  or require an explicit allow-list file.
- **Generated code.** KSP and kapt outputs live outside the source
  tree — do we include generated files in the scan, or trust the
  generator's own invalidation? Probably the former, gated by a
  flag.
- **Transitive vs direct.** The listed symbols are *direct* uses.
  A build tool may want transitive closure (symbol X is used, and
  X uses Y, so Y is also relevant). Leave that to the consumer —
  krit returns direct uses only.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`abi-hash.md`](abi-hash.md), [`symbol-impact-api.md`](symbol-impact-api.md)
