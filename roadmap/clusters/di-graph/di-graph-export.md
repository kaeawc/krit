# DiGraphExport

**Cluster:** [di-graph](README.md) · **Status:** planned · **Severity:** n/a (tool mode)

## Catches

N/A — this is a `krit di-graph` subcommand rather than a rule. Emits
a DOT / JSON / mermaid representation of the project's DI graph
for documentation or review.

## Shape

```
$ krit di-graph :app --format mermaid > app-graph.md
$ krit di-graph :app --format json | jq '.components[0].bindings | length'
$ krit di-graph --format dot > di-graph.dot
```

Proposed CLI shape:

- Parse the subcommand in [`cmd/krit/main.go`](/Users/jason/kaeawc/krit/cmd/krit/main.go) alongside the existing verb handling for `baseline-audit`.
- Accept one optional positional module selector (`:app`, `:feature:data`, etc.). Reuse [`internal/module.ModuleGraph`](/Users/jason/kaeawc/krit/internal/module/graph.go)'s `Modules` map and `FileToModule()` to filter bindings to the requested Gradle module.
- Accept `--format=json|dot|mermaid`, with JSON as the easiest first ship because the graph already has stable structs in [`internal/di/graph.go`](/Users/jason/kaeawc/krit/internal/di/graph.go): `Graph`, `Binding`, and `Dependency`.
- Emit bindings keyed by fully-qualified name, including `modulePath`, `scope`, `file`, `line`, and outbound dependency edges. DOT / mermaid can be thin formatters over the same in-memory payload.

## Dispatch

The cheapest implementation path is:

1. Discover modules with `module.DiscoverModules(scanRoot)` in [`internal/module/discover.go`](/Users/jason/kaeawc/krit/internal/module/discover.go).
2. Populate module-to-module edges with `module.ParseAllDependencies(graph)` in [`internal/module/depparse.go`](/Users/jason/kaeawc/krit/internal/module/depparse.go) so exported JSON can include both binding edges and Gradle-module context.
3. Collect Kotlin sources with `scanner.CollectKotlinFiles(paths, nil)` and parse them with `scanner.ScanFiles(paths, workers)` from [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go).
4. Build the DI graph with `di.BuildGraph(files, moduleGraph)` in [`internal/di/graph.go`](/Users/jason/kaeawc/krit/internal/di/graph.go).
5. Filter the resulting `graph.Bindings` by `Binding.ModulePath` when the user passes a module argument, then hand that filtered graph to a formatter layer.

This should not invent a separate walker. The same `di.BuildGraph` output is the natural substrate for this export subcommand, `whole-graph-binding-completeness`, `di-cycle-detection`, and `dead-bindings`.

## Infra Reuse

- The existing module-aware analysis setup in [`cmd/krit/main.go`](/Users/jason/kaeawc/krit/cmd/krit/main.go) already shows the right ordering: `module.DiscoverModules(...)`, then `module.ParseAllDependencies(...)`, then downstream analysis.
- [`internal/di/graph.go`](/Users/jason/kaeawc/krit/internal/di/graph.go) already normalizes constructor-injected bindings, resolves imports/package-local references via `resolveType(...)`, records source locations, and tags each binding with `ModulePath`.
- [`internal/di/scope.go`](/Users/jason/kaeawc/krit/internal/di/scope.go) already gives exportable lifecycle metadata through `ResolveScope(...)` and `Scope`.
- [`internal/di/graph_test.go`](/Users/jason/kaeawc/krit/internal/di/graph_test.go) is the closest existing fixture for expected graph shape across multiple Gradle modules; future subcommand tests should reuse that mini-project pattern.
- Formatter code should be new, but isolated: for example `internal/di/export_json.go`, `internal/di/export_dot.go`, and `internal/di/export_mermaid.go`, each consuming `*di.Graph` without re-parsing files.

## Links

- Parent: [`../README.md`](../README.md)
