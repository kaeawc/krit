# ApiMigrationAssist

**Cluster:** [sdlc/migration](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

"Retrofit bumped from X to Y; the following usages need attention."
A config-driven migration map that turns deprecated symbol usage
into actionable suggestions with proposed rewrites.

## Shape

```
$ krit migrate --library retrofit --from 2.9.0 --to 2.10.0 --format plain
app/src/main/kotlin/com/example/Api.kt:42:13
  symbol: retrofit2.adapter.rxjava2.RxJava2CallAdapterFactory.create
  current: Retrofit.Builder().addCallAdapterFactory(RxJava2CallAdapterFactory.create())
  suggested: Retrofit.Builder().addCallAdapterFactory(RxJava3CallAdapterFactory.create())
  reason: Retrofit 2.10.0 migration map marks RxJava2 adapter usage as deprecated in favor of RxJava3.
```

`--format json` should mirror the same data as a machine-readable report so CI or editor tooling can group suggestions by library and symbol.

## Dispatch

1. Load the project config and any user-supplied migration map path through `LoadConfig()` or `LoadAndMerge()` in `internal/config/config.go`.
2. Enumerate project inputs with `CollectKotlinFiles()` and `CollectJavaFiles()` from `internal/scanner/scanner.go`, honoring the same exclude semantics as normal analysis.
3. Parse sources via `ScanFiles()` and `ScanJavaFiles()` from `internal/scanner/scanner.go`, then build the cross-file usage index with `BuildIndex()` in `internal/scanner/index.go`.
4. For symbol-level matching, reuse `NewResolver()` from `internal/typeinfer/api.go` and the parallel indexing path in `internal/typeinfer/parallel.go` (`IndexFileParallel()` / `IndexFilesParallel()`) so imported simple names can be normalized before matching against migration entries.
5. Render suggestions through the existing output layer: `FormatPlain()` in `internal/output/output.go` for terminal output and `FormatJSON()` in `internal/output/json.go` for structured automation.

## Infra reuse

- Cross-file reference index from `internal/scanner/index.go`, especially `BuildIndex()`, `CodeIndex.ReferenceFiles()`, and the raw `CodeIndex.References` slice for candidate occurrence enumeration.
- Config loading from `internal/config/config.go`; a first pass can read migration-map metadata from the existing YAML loader via `LoadConfig()` plus `(*Config).Data()` before introducing a typed migration schema.
- Import and type normalization from `internal/typeinfer`: `ImportTable.Resolve()` in `internal/typeinfer/resolver.go` and the resolver built by `NewResolver()` in `internal/typeinfer/api.go`.
- Output formatting reuse from `internal/output/output.go` and `internal/output/json.go` rather than inventing a separate printer for the first cut.
- New: a small typed migration-map model/loader that translates `library + from + to` into symbol rewrite entries; that is the main missing infra rather than a new scanner.

## Links

- Parent: [`../README.md`](../README.md)
