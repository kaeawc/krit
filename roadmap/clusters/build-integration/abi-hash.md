# AbiHash

**Cluster:** [build-integration](README.md) · **Status:** planned · **Severity:** n/a (tool mode)

## Catches

Unnecessary recompilation of downstream modules when an edit to a
Kotlin file does not change its public ABI. A build tool asks krit
for a deterministic hash over a file's (or module's) public surface
and uses it as part of a compile-action cache key. Edits to function
bodies, private helpers, comments, or formatting yield the same hash;
edits to a public signature, annotation, or default value change it.

## Shape

```
$ krit abi-hash :core
:core  3fa1c2d9e7b85af4

$ krit abi-hash app/src/main/kotlin/com/acme/Foo.kt
app/src/main/kotlin/com/acme/Foo.kt  a91b00de2c445100

$ krit abi-hash --json :core
{"module":":core","hash":"3fa1c2d9e7b85af4","inputs":42}
```

A consumer (e.g. grit's `internal/configmodel/action_keys.go`)
mixes the returned hash into its compile-action cache key. If the
hash is stable across an edit, the build tool can skip recompilation
of the downstream graph even though the raw file bytes changed.

## Dispatch

Walk `class_declaration`, `object_declaration`, `function_declaration`,
and `property_declaration` nodes and filter by visibility. For each
kept declaration, normalize a signature record:

- fully qualified name
- visibility modifiers and `open`/`final`/`abstract`/`sealed`
- type parameters with bounds
- parameter types (resolved through typeinfer), names *excluded*
- parameter default presence (bool), value *excluded*
- return type (resolved)
- annotations whose retention is `BINARY` or `RUNTIME`
- companion-object membership
- `inline`/`crossinline`/`noinline`, `suspend`, `operator`, `infix`,
  `tailrec`, `external`

Sort records by FQN + overload discriminator, serialize to a canonical
byte form, hash with SHA-256, and return the first 8 bytes hex-encoded.

The declaration walk and visibility filter already exist in
`collectDeclarations(...)` and `getVisibility(...)` in
[`internal/scanner/index.go`](/Users/jason/kaeawc/krit/internal/scanner/index.go),
shared with
[`architecture/public-api-surface-snapshot.md`](../architecture/public-api-surface-snapshot.md).
ABI hashing is the machine-readable sibling: same walk, no textual
rendering, output is a hash rather than a sorted diff-friendly file.

## Infra Reuse

- Module/file targeting: `module.DiscoverModules(rootDir)` in
  [`internal/module/discover.go`](/Users/jason/kaeawc/krit/internal/module/discover.go)
  and `(*ModuleGraph).FileToModule(...)` in
  [`internal/module/graph.go`](/Users/jason/kaeawc/krit/internal/module/graph.go).
- Kotlin discovery and parse: `scanner.CollectKotlinFiles(...)` and
  `scanner.ScanFiles(...)` in
  [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go).
- Declaration walk: `scanner.BuildIndex(...)` /
  `collectDeclarations(...)` in
  [`internal/scanner/index.go`](/Users/jason/kaeawc/krit/internal/scanner/index.go).
- Type resolution for parameter/return types: typeinfer's public
  entry points in
  [`internal/typeinfer/api.go`](/Users/jason/kaeawc/krit/internal/typeinfer/api.go).
- CLI dispatch: follow the `os.Args` rewrite pattern used by
  `runBaselineAudit(...)` in
  [`cmd/krit/baseline_audit.go`](/Users/jason/kaeawc/krit/cmd/krit/baseline_audit.go).
- Caching of computed hashes keyed by file content hash:
  [`internal/cache/cache.go`](/Users/jason/kaeawc/krit/internal/cache/cache.go).

## Open questions

- **Annotation scope.** `@JvmStatic`, `@JvmName`, `@Throws`, and
  `@Deprecated` change the binary surface; `@VisibleForTesting` does
  not. Start with a hard-coded allowlist, revisit once there is a
  concrete consumer.
- **Inline function bodies.** `inline fun` bodies are part of the
  ABI because callers inline them at compile time. Either include
  the body bytes in the hash for `inline` functions only, or mark
  the module as "cannot use ABI cache" when it exposes any inline.
- **Stable type identity across typeinfer upgrades.** If krit's
  type normalizer changes, every hash changes even though nothing
  in the source did. Version the hash format and include the version
  byte in the output so consumers know to invalidate on krit upgrade.

## Links

- Parent: [`../README.md`](../README.md)
- Sibling (human-readable form): [`../architecture/public-api-surface-snapshot.md`](../architecture/public-api-surface-snapshot.md)
- Related: [`used-symbol-extraction.md`](used-symbol-extraction.md)
