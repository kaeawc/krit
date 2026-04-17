# PublicApiSurfaceSnapshot

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** n/a (tool mode)

## Catches

Unintended changes to a module's public API surface. Not a rule in
the traditional sense — a `krit api-snapshot` subcommand that
serializes the public signature set, and `krit api-diff` that
fails if the committed snapshot differs from the current source.

## Shape

```
$ krit api-snapshot :core > core-api.txt
$ git add core-api.txt && git commit -m 'baseline'
# later
$ krit api-diff :core core-api.txt
core: added public fun newInternalHelper(): Foo
```

Dispatch this as a real verb in [`cmd/krit/main.go`](/Users/jason/kaeawc/krit/cmd/krit/main.go),
using the same early-`os.Args` rewrite pattern that `baseline-audit` already
uses, with the verb bodies split into focused helpers beside
[`cmd/krit/baseline_audit.go`](/Users/jason/kaeawc/krit/cmd/krit/baseline_audit.go).
The cheapest first shape is:

- `krit api-snapshot <module-or-path>` writes a stable newline-sorted text snapshot to stdout.
- `krit api-diff <module-or-path> <snapshot-file>` compares current output to a committed snapshot file and exits non-zero on drift.
- `<module-or-path>` accepts a Gradle path like `:core` or a filesystem path; Gradle paths resolve through the existing module graph, plain paths reuse the normal scan-root behavior.

## Dispatch

`class_declaration`/`function_declaration`/`property_declaration`
walk with visibility filter; normalized signature output format.
Analogous to JetBrains'
`binary-compatibility-validator` plugin but pure tree-sitter.

The existing declaration indexer in
[`internal/scanner/index.go`](/Users/jason/kaeawc/krit/internal/scanner/index.go)
already walks these nodes in `collectDeclarations(...)` and classifies
visibility in `getVisibility(...)`. That should be the starting seam rather
than a brand-new walker. A follow-up implementation can either:

- extend `scanner.Symbol` with the extra API-snapshot fields needed for stable rendering, or
- add a sibling exported collector in `internal/scanner/` that reuses the same node-type coverage and visibility logic but records richer signature data.

For signature normalization, reuse the existing AST helpers in
[`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go)
such as `FindChild(...)` / `HasModifier(...)`, and mirror the parameter/default
extraction patterns already present in
[`internal/rules/declaration_summary.go`](/Users/jason/kaeawc/krit/internal/rules/declaration_summary.go)
so the snapshot can distinguish overloads, property mutability, and constructor
surface without pulling in rule dispatch.

## Infra Reuse

The subcommands should build on the existing analysis pipeline instead of
introducing a separate project model:

- Module targeting: `module.DiscoverModules(rootDir)` in [`internal/module/discover.go`](/Users/jason/kaeawc/krit/internal/module/discover.go) plus `(*ModuleGraph).FileToModule(...)` in [`internal/module/graph.go`](/Users/jason/kaeawc/krit/internal/module/graph.go) to resolve `:core` and keep snapshot scope aligned with Krit's module-aware analysis.
- Kotlin file discovery and parsing: `scanner.CollectKotlinFiles(...)` and `scanner.ScanFiles(...)` in [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go).
- Existing symbol/index plumbing: `scanner.BuildIndex(...)` in [`internal/scanner/index.go`](/Users/jason/kaeawc/krit/internal/scanner/index.go), with API rendering layered on top of the declarations it already collects.
- Diff/report plumbing: follow the plain-text/JSON split used by `runBaselineAudit(...)` in [`cmd/krit/baseline_audit.go`](/Users/jason/kaeawc/krit/cmd/krit/baseline_audit.go) so `api-diff` can print human-readable drift by default but still emit machine-readable output later without redesign.

## Links

- Parent: [`../README.md`](../README.md)
