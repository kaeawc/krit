# CrossModuleDeadCode

**Cluster:** [build-integration](README.md) · **Status:** planned · **Severity:** n/a (tool mode)

## Catches

Unused public classes, functions, properties, and resources at
project scale. Krit's current `internal/deadcode/` pass is
file-local: a `private` helper with no callers in its own file is
caught, but a `public` function with no callers in the *repository*
is not — because that check requires reasoning across the module
graph. `krit dead-code --project` treats every source entry point
(main functions, Android components, test functions, JSR-330 /
Hilt entry points, reflection roots supplied by the caller) as a
root and reports every declaration unreachable from them.

## Shape

```
$ krit dead-code --project
com.acme.core/src/main/kotlin/Foo.kt:42
  public fun parseLegacyConfig(raw: String): Config  [no callers]
com.acme.feature-auth/src/main/kotlin/AuthFlow.kt:117
  internal class OldOAuthHandler  [no callers]

$ krit dead-code --project --json
[{"file":"...","line":42,"fqn":"com.acme.core.parseLegacyConfig","kind":"function","reason":"no-callers"}]
```

A build tool (or a reviewer) uses the output as removal candidates.
The pairing with `krit remove-dead-code` (already exposed in
[`cmd/krit/remove_dead_code.go`](/Users/jason/kaeawc/krit/cmd/krit/remove_dead_code.go))
is deliberate: discover with `dead-code --project`, apply with
`remove-dead-code`.

## Dispatch

Two-phase reachability:

1. **Root discovery.** Scan for entry points:
   - functions named `main` with the expected signature
   - classes annotated `@HiltAndroidApp`, `@AndroidEntryPoint`,
     `@HiltViewModel`, `@HiltWorker`
   - `Activity` / `Service` / `BroadcastReceiver` /
     `ContentProvider` subclasses declared in
     `AndroidManifest.xml`
   - test functions annotated `@Test`
   - JVM service-loader entries under `META-INF/services/`
   - `public` declarations in modules marked as "library boundary"
     (explicit config, default off)
   - user-supplied extra roots via `--root` or a config file

2. **Reachability walk.** Starting from the root set, follow the
   call graph and type-reference graph across module boundaries.
   Every declaration reached is live. Every declaration not
   reached is reported.

The walk reuses the same oracle-driven resolution as
[`used-symbol-extraction.md`](used-symbol-extraction.md) — both
features need "for this reference, what declaration does it point
to?" The difference is direction: used-symbol extraction collects
outgoing references per unit; dead-code collects incoming callers
per declaration and then computes reachability.

## Infra Reuse

- File-level dead code already exists in
  [`internal/deadcode/removal.go`](/Users/jason/kaeawc/krit/internal/deadcode/removal.go).
  Keep the file-level pass as the fast default; add a project-level
  entry point alongside it.
- Module graph: `module.DiscoverModules(...)` and `ModuleGraph`
  in
  [`internal/module/discover.go`](/Users/jason/kaeawc/krit/internal/module/discover.go)
  /
  [`internal/module/graph.go`](/Users/jason/kaeawc/krit/internal/module/graph.go).
- Cross-file index: `scanner.BuildIndex(...)` in
  [`internal/scanner/index.go`](/Users/jason/kaeawc/krit/internal/scanner/index.go).
- Symbol resolution: oracle +
  [`internal/typeinfer/api.go`](/Users/jason/kaeawc/krit/internal/typeinfer/api.go).
- CLI wiring: existing
  [`cmd/krit/remove_dead_code.go`](/Users/jason/kaeawc/krit/cmd/krit/remove_dead_code.go)
  already dispatches the file-level flavor; add a `--project`
  flag or sibling verb.

## Open questions

- **Reflection roots.** Same problem as
  [`used-symbol-extraction.md`](used-symbol-extraction.md).
  Proposed answer: a `krit.roots.toml` file at the repo root lists
  FQNs that must always be treated as reachable. Users maintain it
  by hand; krit does not attempt to infer it.
- **Serialization / generated code.** A class used only as
  `kotlinx.serialization` output is reachable via a generated
  serializer. Include generated sources in the scan or treat every
  `@Serializable` class as a root.
- **Public library modules.** A module whose purpose is to be
  consumed by external callers (published AAR, SDK) has no
  in-repo callers by design. Mark these modules explicitly and
  treat all their `public` symbols as roots.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`symbol-impact-api.md`](symbol-impact-api.md), [`used-symbol-extraction.md`](used-symbol-extraction.md)
