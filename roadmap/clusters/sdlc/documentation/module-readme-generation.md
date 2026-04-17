# ModuleReadmeGeneration

**Cluster:** [sdlc/documentation](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit gen module-readme :core` — per-module: public API list,
dependency summary, fan-in/fan-out, test-file list. Template output.

## Shape

```
$ krit gen module-readme :core > core/README.md
```

CLI shape:

- Resolve the project root from the first positional path, defaulting to `.`
- Accept a Gradle module path exactly as `module.DiscoverModules()` reports it
  (`:core`, `:feature:payments`, etc.)
- Emit markdown to stdout in the first iteration so shell redirection stays the
  write path
- Fail clearly when the module is unknown or no `settings.gradle(.kts)` file is
  present

Output format (markdown):

```markdown
# :core

**Depends on:** :common, :platform-api
**Depended on by:** :feature:foo, :feature:bar (4 modules)

## Public API

- class `UserRepository`
  - `fun get(id: Long): User`
  - `fun save(user: User)`

## Tests

- `test/UserRepositoryTest.kt` (7 tests)
```

The first implementation can keep the template narrow:

- `Depends on` from `graph.Modules[modulePath].Dependencies`
- `Depended on by` from `graph.Consumers[modulePath]`
- `Public API` from module-local `scanner.Symbol` entries filtered to
  `Visibility == "public"`
- `Tests` from module-local files whose paths match existing internal
  test-source heuristics

## Dispatch

- Follow the explicit subcommand entry pattern already used by
  `baseline-audit` in [cmd/krit/main.go](/Users/jason/kaeawc/krit/cmd/krit/main.go:40),
  then hand off to a focused helper instead of expanding the main scan path
- Internal call order should be:
  `module.DiscoverModules()` in [internal/module/discover.go](/Users/jason/kaeawc/krit/internal/module/discover.go:15),
  `module.ParseAllDependencies()` in [internal/module/depparse.go](/Users/jason/kaeawc/krit/internal/module/depparse.go:37),
  `scanner.CollectKotlinFiles()` and `scanner.ScanFiles()` in [internal/scanner/scanner.go](/Users/jason/kaeawc/krit/internal/scanner/scanner.go:128),
  then `module.BuildPerModuleIndex()` in [internal/module/permodule.go](/Users/jason/kaeawc/krit/internal/module/permodule.go:23)
- Module selection should stay data-driven: look up `graph.Modules[modulePath]`
  directly and let `BuildPerModuleIndex()` reuse `graph.FileToModule()` for
  assignment instead of inferring ownership ad hoc
- Keep markdown rendering as the final step: gather a small summary struct from
  internal data first, then format it

## Infra reuse

- Module graph:
  `module.DiscoverModules()` builds `ModuleGraph.Modules` plus source roots, and
  `module.ParseAllDependencies()` fills `Module.Dependencies`,
  `Module.IsPublished`, and `ModuleGraph.Consumers`
- Per-module indexing:
  `module.BuildPerModuleIndex()` exposes both `PerModuleIndex.ModuleFiles` and
  `PerModuleIndex.ModuleIndex`, so README generation can stay scoped to one
  module without rescanning the project repeatedly
- Public API extraction:
  `scanner.BuildIndex()` / `scanner.BuildIndexFromData()` in
  [internal/scanner/index.go](/Users/jason/kaeawc/krit/internal/scanner/index.go:67)
  already collect `scanner.Symbol{Name, Kind, Visibility, File, Line}` for
  functions, classes, objects, interfaces, and properties; the subcommand can
  start by filtering `pmi.ModuleIndex[modulePath].Symbols` to public
  declarations, then add `scanner.WalkNodes()` from
  [internal/scanner/scanner.go](/Users/jason/kaeawc/krit/internal/scanner/scanner.go:375)
  later if it needs richer per-class method grouping
- Fan-in / fan-out:
  module-level fan-out already exists as `Module.Dependencies`, module-level
  fan-in already exists as `ModuleGraph.Consumers`, and optional class-level
  hotspot enrichment can reuse `CodeIndex.ClassLikeFanInStats()` in
  [internal/scanner/hotspot.go](/Users/jason/kaeawc/krit/internal/scanner/hotspot.go:14)
- Test-file list:
  reuse the path rules already encoded in `isTestFile()` in
  [internal/rules/naming.go](/Users/jason/kaeawc/krit/internal/rules/naming.go:378)
  as the baseline filter for `pmi.ModuleFiles[modulePath]`; if the README later
  wants only canonical Gradle test roots, narrow with
  `shouldSkipPackageDependencyCycleFile()` in
  [internal/rules/package_dependency_cycle.go](/Users/jason/kaeawc/krit/internal/rules/package_dependency_cycle.go:122)

## Links

- Parent: [`../README.md`](../README.md)
