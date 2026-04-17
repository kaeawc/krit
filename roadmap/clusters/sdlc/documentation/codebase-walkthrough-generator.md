# CodebaseWalkthroughGenerator

**Cluster:** [sdlc/documentation](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit gen walkthrough` — pick N representative files and emit a
guided reading doc.

## Heuristic

Start with the highest-fan-in class in the project, then follow
its public collaborators, ranked by reference count, up to N files.

## Shape

```
$ krit gen walkthrough --n 10 > docs/walkthrough.md

# default plain-text report
Seed: com.example.UserRepository (app/src/main/kotlin/com/example/UserRepository.kt)
Why this file: highest class-like fan-in (138 external files)

Reading order:
1. app/src/main/kotlin/com/example/UserRepository.kt
   public API referenced from 138 files
2. app/src/main/kotlin/com/example/SqlUserStore.kt
   collaborator referenced from 44 seed call sites
3. app/src/main/kotlin/com/example/UserMapper.kt
   collaborator referenced from 31 seed call sites
```

Prefer a second machine-readable mode for editor / docs automation:

```
$ krit gen walkthrough --n 10 --report json
```

## Dispatch

- Parse the verb in `cmd/krit/main.go` the same way `baseline-audit` is
  detected today, then hand off to a dedicated helper such as
  `runWalkthrough(...)` in a new `cmd/krit/walkthrough.go`.
- File collection should reuse `scanner.CollectKotlinFiles(...)` and
  `scanner.CollectJavaFiles(...)` from
  `internal/scanner/scanner.go`, so the walkthrough sees the same Kotlin /
  Java universe as cross-file analysis.
- Parsing should reuse `scanner.ScanFiles(...)` and
  `scanner.ScanJavaFiles(...)` from `internal/scanner/scanner.go`.
- The first implementation should build one global reference index with
  `scanner.BuildIndexWithTracker(...)` from `internal/scanner/index.go`,
  not a bespoke graph builder.

## Infra reuse

- Cross-file reference index (same data as
  [`../pr-workflow/blast-radius-scoring.md`](../pr-workflow/blast-radius-scoring.md)).
- Seed-file selection should call
  `(*scanner.CodeIndex).ClassLikeFanInStats(true)` from
  `internal/scanner/hotspot.go` and pick the first entry. That already
  computes "highest-fan-in class-like declaration" and filters out the
  declaring file from the fan-in set.
- Collaborator ranking can reuse
  `(*scanner.CodeIndex).ReferenceFiles(name)`,
  `(*scanner.CodeIndex).ReferenceCount(name)`, and
  `(*scanner.CodeIndex).CountNonCommentRefsInFile(name, file)` from
  `internal/scanner/index.go` to score files that co-occur with the seed's
  public symbol names.
- If the report groups the walkthrough by module, reuse
  `module.DiscoverModules(...)` from `internal/module/discover.go`,
  `module.BuildPerModuleIndexWithGlobal(...)` from
  `internal/module/permodule.go`, and
  `(*module.ModuleGraph).FileToModule(...)` from `internal/module/graph.go`
  instead of inferring module ownership again.
- Output can stay plain-text first; no new formatter layer is required for
  v1 because the helper can emit directly once it has the ordered file list.

## Links

- Parent: [`../README.md`](../README.md)
