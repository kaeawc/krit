# Module compilation requests

`--modules-file request.json` prints the check response to stdout (or accepts
`--output FILE`). The daemon accepts `command: "analyzeModules"`; `check` also
accepts `modules`. Both reuse `rules`, `ruleConfigs`, `testFiles`, and `scanPaths`.
`checkFiles` contains path strings. An absent/empty modules list uses the legacy
session compilation. The CLI JSON need not contain `id` or `command`.

Each module supplies `id`, `platform` (`jvm` or `android`), `kind`,
`sourceRoots`, `generatedSourceRoots`, ordered `classpath`, `dependsOn`,
`friends`, `compilerArgs`, `jvmTarget`, and optional `fragments`. A fragment has
`name`, `sourceRoots`, and `refines`. Sources are read from disk on every request.
Dependencies and friends must identify modules in the same request.

The response retains the existing file-keyed `errorFiles`, `crashed`,
`ruleErrors`, and `findings`, and adds `modules: [{id, mode, firstError}]` and
`decidingModules: {file: moduleId}`. The deepest matching source root decides ownership; declaration order
breaks ties for identical shared roots. Files outside all modules are explicitly gated. Module
status is not a file verdict: a gated compilation can still authoritatively
check files with no errors. `merged-fallback` similarly describes the source
merge, while file error maps determine authoritativeness.

Outputs remain under one temporary session directory until disposal. Compiles
are sequential, including SCC cycle fallbacks. Each cyclic component uses one
legacy union compile and reports `skipped` plus the cycle reason. Unchanged
modules return cached verdicts with their original mode. Compile fingerprints include full source contents, classpath identity, compiler
arguments, fragments, and upstream compile fingerprints. Checker context has a
separate, module-local key; findings-only reruns do not invalidate dependents.
Classpath file hashes are memoized by canonical path, size, and modification time.
Dependency binaries are supplied in upstream-first order, including transitive
signatures; friend access itself stays direct-only.

Owned-source errors determine gating independently of merged-source errors.
K2 emits no binaries when any frontend source has errors, so clean fallback
modules may still require source replacement downstream. These source sets are
rebuilt from unique graph IDs, not concatenated from previous merges. Above 300
upstream source files, recovery switches to legacy union compilation and reports
`legacy-fallback` with the limit reason.

Outputs are published by atomic rename from a fresh temporary generation.
Unreferenced generations are removed after requests, including removed modules
and changed components. Legacy session rebuilds transfer module cache ownership.
Session directory names encode the process ID; startup sweeps directories older
than 24 hours only when their owning process is no longer alive.

## Verified HMPP arguments (Kotlin 2.3.21)

The embedded compiler's `CommonCompilerArguments` field annotations, inspected
with `javap -v -p -classpath kotlin-compiler-2.3.21.jar`, declare:

```
-Xmulti-platform
-Xfragments=<fragment name>[,<fragment name>...]
-Xfragment-sources=<fragment name>:<path>
-Xfragment-refines=<fromModuleName>:<onModuleName>
```

The driver repeats source mappings per source **file**, matching the exact
free-argument spelling, and refinement mappings per edge. The real embedded
compiler tests cover common `expect` and JVM `actual` resolution. This is the
CLI compiler pipeline, without the separate KMP/klib compilation mode or KAA.
Legacy compiles retain the existing expect-only source-root heuristic.

## Compilation seam and retained outputs

`AnalysisSession.compileModule` is the single K2 execution seam for both checker
and oracle compilation. Callers supply compiler arguments (including destination
and backend options), diagnostic collectors, and registry context scopes. Contexts
are unwound after execution, including failures. Combining oracle and checker
collection and selecting a frontend-only backend remain follow-up work.

Cached module results retain their output directories and class files until
replacement or session disposal. In particular, a cache hit never clears outputs
for a `dependsOn` or `friends` target. A downstream-only edit therefore reuses the
same upstream binaries; gated upstream directories remain available to the merged
fallback too. The three-module regression checks physical class contents after
such an edit for both dependency and friend edges.
