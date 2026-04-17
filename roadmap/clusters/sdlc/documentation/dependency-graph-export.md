# DependencyGraphExport

**Cluster:** [sdlc/documentation](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit graph` — emits a module or package dependency graph as DOT /
JSON / mermaid.

## Shape

```
$ krit graph --format mermaid > docs/modules.md
$ krit graph --format dot | dot -Tsvg > modules.svg
$ krit graph --scope package --format json > package-graph.json
```

Proposed CLI shape:

- Parse `graph` in [`cmd/krit/main.go`](/Users/jason/kaeawc/krit/cmd/krit/main.go) alongside the existing verb handling for `baseline-audit`.
- Accept `--format=json|dot|mermaid`, with JSON as the first ship because the module graph already has stable structs in [`internal/module/graph.go`](/Users/jason/kaeawc/krit/internal/module/graph.go): `ModuleGraph`, `Module`, and `Dependency`.
- Accept `--scope=module|package`, defaulting to `module`. Module scope is the cheapest first implementation because `internal/module` already discovers modules and project-to-project edges. Package scope can follow by reusing the import graph logic already embedded in [`internal/rules/package_dependency_cycle.go`](/Users/jason/kaeawc/krit/internal/rules/package_dependency_cycle.go).
- Accept one optional positional root path, matching the normal analyzer entrypoint and feeding directly into `module.DiscoverModules(...)` and `scanner.CollectKotlinFiles(...)`.

## Dispatch

The cheapest implementation path is:

1. Detect the `graph` verb early in [`cmd/krit/main.go`](/Users/jason/kaeawc/krit/cmd/krit/main.go), the same way `baseline-audit` is peeled off `os.Args` today.
2. For `--scope=module`, build the graph with `module.DiscoverModules(scanRoot)` from [`internal/module/discover.go`](/Users/jason/kaeawc/krit/internal/module/discover.go), then enrich edges with `module.ParseAllDependencies(graph)` from [`internal/module/depparse.go`](/Users/jason/kaeawc/krit/internal/module/depparse.go).
3. Marshal that `*module.ModuleGraph` into JSON directly, and layer DOT / mermaid formatters over the same in-memory payload.
4. For `--scope=package`, collect Kotlin files with `scanner.CollectKotlinFiles(paths, nil)` and parse them with `scanner.ScanFiles(paths, workers)` from [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go).
5. Reuse `packageDependencyCycleData(...)` from [`internal/rules/package_dependency_cycle.go`](/Users/jason/kaeawc/krit/internal/rules/package_dependency_cycle.go) to derive package/import edges, but emit the full adjacency graph instead of only SCC findings. `findPackageDependencyCycles(...)` in the same file is the right validation helper for an optional `--cycles-only` follow-up, not a required part of first ship.

This should not invent a second Gradle parser or a second package-import walker. The first implementation can stay thin by serializing existing graph-building outputs.

## Infra reuse

- [`internal/module/discover.go`](/Users/jason/kaeawc/krit/internal/module/discover.go) already handles `settings.gradle(.kts)` parsing through `DiscoverModules(...)`, `parseIncludes(...)`, and `parseProjectDirOverrides(...)`.
- [`internal/module/depparse.go`](/Users/jason/kaeawc/krit/internal/module/depparse.go) already resolves Gradle project dependencies through `ParseAllDependencies(...)`, `parseDeps(...)`, and `typesafeAccessorToPath(...)`, including `projects.fooBar` accessors.
- [`internal/module/graph.go`](/Users/jason/kaeawc/krit/internal/module/graph.go) already provides the stable export substrate: `ModuleGraph.Modules`, `ModuleGraph.Consumers`, and `Module.Dependencies`.
- [`internal/rules/package_dependency_cycle.go`](/Users/jason/kaeawc/krit/internal/rules/package_dependency_cycle.go) already contains the package-edge extraction helpers `packageDependencyCycleData(...)` and `shouldSkipPackageDependencyCycleFile(...)`; package export should build on those helpers instead of reparsing imports differently.
- [`internal/module/discover_test.go`](/Users/jason/kaeawc/krit/internal/module/discover_test.go), [`internal/module/depparse_test.go`](/Users/jason/kaeawc/krit/internal/module/depparse_test.go), and [`internal/module/graph_test.go`](/Users/jason/kaeawc/krit/internal/module/graph_test.go) already define the mini-project cases that a future `krit graph` test should mirror.
- Formatter code would be new but isolated, for example `internal/module/export_json.go`, `internal/module/export_dot.go`, and `internal/module/export_mermaid.go`, each consuming `*module.ModuleGraph` or a shared package-graph payload without touching rule execution.

## Links

- Parent: [`../README.md`](../README.md)
- Related: `roadmap/clusters/di-graph/di-graph-export.md`
