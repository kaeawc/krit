# Module sharding (experimental)

Krit's type oracle normally compiles the whole repository in one JVM. Module
sharding is an opt-in alternative for Gradle projects: it analyzes each module
on its own, in several worker JVMs, against the dependency jars Gradle already
built, and merges the per-module facts into the same oracle file. The default
is unchanged; nothing uses sharding unless you ask for it.

## When it is used

Sharding applies only when all of these hold. Otherwise krit logs the reason
(with `-v`) and runs the whole-repository compile.

- `KRIT_MODULE_SHARDS` is set to a worker count or `auto`.
- A Gradle model exists under `<repo>/.krit/gradle-model` (run the
  `kritExportModel` task first).
- The modules' dependencies have build outputs. A project dependency resolves
  to its library jar (`build/intermediates/compile_library_classes_jar/release/...`
  or `build/libs/<name>.jar`). A dependency without a jar is compiled from its
  sources together with the module, up to 3000 files per module. Past that
  limit the dependency stays unresolved and the module's files get no type
  facts for it.
- At most 0.5% of the Kotlin files krit scans lack facts. Files outside every
  Gradle source set are analyzed in extra per-directory units first; if too
  many remain uncovered, the run is discarded.

## Switches

| Variable | Meaning |
| --- | --- |
| `KRIT_MODULE_SHARDS=auto` | Use sharding with the default worker count: half the CPUs, between 2 and 8. |
| `KRIT_MODULE_SHARDS=<n>` | Use sharding with `n` modules compiling at a time. `1` compiles one module at a time, which is slower than the whole compile. |
| `KRIT_MODULE_SHARDS=0` or unset | Whole-repository compile (default). With `--fir`, the source sets holding a file the compile gates on a duplicate declaration between source sets are rechecked one source set at a time (see [Kotlin compiler settings](kotlin-compiler-settings.md#known-limits)). |
| `KRIT_MODULE_SHARDS=merged` | Whole-repository compile with no source-set recheck and no settings-group sessions (see [Kotlin compiler settings](kotlin-compiler-settings.md#settings-group-sessions---fir)). |
| `KRIT_MODULE_SHARDS=isolate` | Compile every scanned source set on its own (see [Kotlin compiler settings](kotlin-compiler-settings.md#known-limits)). |
| `KRIT_WORKER_AOT=0` | Do not use a JDK 25+ ahead-of-time cache for the workers. |
| `KRIT_WORKER_JAVA` | Path to the `java` the workers use. By default a JDK 25+ is preferred when one is installed. |
| `KRIT_WORKER_THREADS=<n>` | Modules one worker JVM compiles at a time (default 4). `1` starts a JVM per concurrent module. |

The modules compiling at a time run on as few worker JVMs as
`KRIT_WORKER_THREADS` allows: two concurrent modules run on two threads of one
JVM, eight on two JVMs of four threads, and five on JVMs of three and two
threads, with modules shared out in proportion to the threads. Threads of one JVM share its
JIT-compiled compiler and its heap, which a JVM per module would each build up
again. On a 4-core machine the source-set recheck of 24 sets took about 25s
on one JVM with two threads against about 32s on two JVMs, with the same
facts and verdicts (#1656). This applies wherever modules are compiled on
their own: module sharding, PR mode, the source-set recheck and settings-group
sessions.

Each concurrent module compile gets a heap of at most 5 GB, reduced so that
all of them together stay under 40% of the available memory, and never below
1.5 GB; a worker JVM's heap is that times its threads, the same per compile as
a JVM per module. On Linux the
available memory is the cgroup memory limit when one is set, and the CPU count
follows the cgroup CPU quota; see
[Hermetic and build-system runs](hermetic-builds.md#cpu-and-memory).

## Cache

Facts are cached per module in `<repo>/.krit/module-facts`, keyed by the
`krit-fir` jar, the module's compiler settings and plugins, its classpath
(each jar's path and content, and each class directory's files; see
[cache inventory](perf.md#cache-inventory-and-invalidation)), and the content
of every source file. A changed module, or a rebuilt dependency jar,
invalidates only the modules whose key changed. Unreferenced entries are
removed after seven days. The merged oracle is rewritten only when the set of
keys changes.

The FIR checkers run inside the same module compiles. Each module checks its
own requested files, and the responses are merged. A requested file that no
module analyzed is reported as not checked, so the Go rules keep their own
findings for it.

## How results differ from the whole compile

Module analysis sees each dependency as a compiled library, as `kotlinc` does
under Gradle. That changes some facts, usually for the better:

- Smart casts on properties of classes from another module are not allowed by
  Kotlin. A whole-repository compile treats every file as one module and
  allows them, so rules such as `UnnecessarySafeCall` and
  `UnnecessaryNotNullOperator` report findings there that a module compile
  correctly does not.
- Code generated by Gradle (for example from schema or binding generators) is
  present in the jars. The whole compile cannot see it and reports unresolved
  types, which makes some rules abstain.
- Files whose module has dependencies without jars can lose facts the whole
  compile has. Those files fail to resolve and are treated as not authoritative.

Two modes can still differ for the same files, so compare before relying on
sharding. `scripts/modshard-parity.sh <repo> <out-dir> <krit>` runs a
repository cold and warm in both modes and prints findings that appear in only
one (`internal/devtools/findingsdiff`). `internal/devtools/oraclediff`
compares two oracle files expression by expression.

## Limits

- Needs a prior Gradle build of the dependencies; a clean checkout falls back.
- Cold runs use about two to three times the CPU time of the whole compile
  (each worker pays JVM and compiler start-up and compiles the dependency
  closure's declarations again). Wall time is about the same as the whole
  compile on a 14-core machine, and worse on a busy one. Changing one module
  re-analyzes only that module.
- Warm runs spend a few extra seconds hashing sources and merging.
- Memory use grows with the worker count (several GB per worker).
- Modules are analyzed against built jars, so a stale build shows stale facts.
  Rebuild with Gradle after changing a module that others depend on.
