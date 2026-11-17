# Hermetic and build-system runs

This page collects the settings that matter when krit runs inside a build
system action (Bazel, Buck, a sandboxed CI step): no shared state between runs,
no processes that outlive the action, and the same output for the same inputs.

## JDK selection

Krit looks for `java` and `javac` under `$JAVA_HOME/bin` when `JAVA_HOME` is
set, and on `PATH` otherwise. This covers every JVM krit launches (the type
oracle, the FIR checker, the krit-fir daemons, the Java facts helper and its
`javac` compile) and the checks that decide whether to download the krit-fir
jar. A `JAVA_HOME` that lacks the tool is an error, not a fall back to `PATH`,
so an action that pins a JDK through `JAVA_HOME` never runs a different one.
`krit --doctor` prints the launchers it found.

## CPU and memory

Worker counts, including the `-j` default, follow `GOMAXPROCS`. On Linux the
Go runtime sets it from the CPU affinity mask and the cgroup CPU quota, so a
sandbox with a 2-CPU quota on a 64-core host starts 2 workers, not 64. Set
`GOMAXPROCS` or pass `-j` to override it.

On Linux krit reads the cgroup memory limit (v2 `memory.max`, or v1
`memory.limit_in_bytes`, the smallest set on the process's cgroup or its
ancestors) instead of the host's `/proc/meminfo`:

- A scan sets a Go soft memory limit of three quarters of the cgroup limit, so
  the garbage collector works harder before the kernel kills the process. The
  remaining quarter covers tree-sitter's C allocations and the JVMs krit starts
  in the same cgroup. Without a cgroup limit nothing changes.
- `GOMEMLIMIT` replaces the derived soft limit, and `GOMEMLIMIT=off` disables
  it. `GOGC` or `GOMEMLIMIT` also turn off krit's own GC percent tuning, so
  explicit settings always win.
- [Module sharding](module-sharding.md) sizes each worker JVM's heap from the
  cgroup limit rather than the host's memory.

The krit-fir JVMs size their heaps with `-XX:MaxRAMPercentage`, which the JVM
already computes from the container limit.

## Background processes

By default a scan may leave long-lived processes behind so the next run starts
warm: the krit daemon, and krit-fir JVM daemons for the FIR checker and for
small sets of type-oracle cache misses. To keep every process inside the
action, pass:

| Setting | Effect |
| --- | --- |
| `--no-daemon` or `KRIT_NO_DAEMON_AUTOSTART=1` | Run in-process; do not start or contact the krit daemon. |
| `--no-fir-daemon` | Run the FIR checker and type-oracle cache misses one-shot, so no krit-fir daemon outlives the scan. |

`--output-types` (a standalone type-oracle dump) runs its cache misses one-shot
under either `--no-daemon` or `--no-fir-daemon`. It compiles against the same
compile context a scan's oracle uses: the Gradle model's source roots and
classpath, `oracle.classpath` from `krit.yml`, the JVM target and the Kotlin
compiler settings.

## Sharding rules by scope

A build system that runs one action per module gets wrong results from rules
that look across files: dead-code and unused-resource rules, for example,
report a declaration as unused when its only caller is in another module.
`--only-scope` and `--exclude-scope` split the rule set so each module can run
the per-file rules while one whole-repository action runs the rest:

```bash
# One action per module, cacheable on the module's own inputs.
krit --only-scope=file --no-daemon --no-cache -f sarif -o app.sarif app/

# One action for the whole repository, Go-only.
krit --only-scope=project --no-fir --no-daemon --no-cache -f sarif -o project.sarif .
```

Both flags take a comma-separated list. The groups are:

| Group | Rules |
| --- | --- |
| `file` | Rules whose findings for a file depend only on that file and on type information: the per-file AST, line-scanning and per-file aggregate passes. |
| `project` | Every other rule: cross-file and module-index rules, rules over all parsed files, and Android manifest, resource, icon and Gradle rules. |

The individual dispatcher scopes are also accepted: `per-file-node`,
`per-file-all-nodes`, `line-pass`, `aggregate`, `cross-file`,
`module-index`, `parsed-files`, `manifest`, `resource`, `resource-source`,
`icons` and `gradle`. An unknown value exits with status 2. When both flags are
set, `--only-scope` selects and `--exclude-scope` then removes. The two groups
partition the rule set, so a `file` run and a `project` run over the same tree
report the same findings as one unfiltered run.

`krit --list-rules --format json` reports each rule's group in its `scope`
field. File-scope rules that use type information still resolve types from
the files in the scan and from the classpath, so give each module action the
module's compile classpath (see [FIR scan requirements](configuration.md#fir-scan-requirements)).
The scope flags, like `--max-cost`, keep the scan in-process instead of
delegating it to a krit daemon.

## Reproducible output

SARIF, Checkstyle and plain output depend only on the inputs. JSON output also
carries `durationMs`, and cache statistics and `--perf` timings when they are
collected, so two runs over the same tree differ. Pass `--reproducible` to drop
them: `durationMs` is written as `0`, and `cache`, `caches`, `cacheBudget`,
`perfTiming` and `perfRuleStats` are omitted. Suggested-fix edits then name
their `targetFile` relative to `--base-path`, like finding paths, instead of
by absolute path. The result is byte-identical across cold and warm caches,
worker counts, and checkouts at different paths. `--perf` still prints its
summary on stderr. A `--reproducible` scan stays in-process instead of
delegating to a krit daemon.

To keep a build-system action cacheable:

- Pass `--no-cache`. The incremental cache stores absolute paths and
  modification times, so it cannot move between machines; the build system's
  own action cache replaces it.
- Point `KRIT_STATE_DIR`, `KRIT_HOME`, `HOME` and `TMPDIR` at directories the
  action owns (see [Cache location](configuration.md#cache-location)).
- Set `KRIT_FIR_JAR` to a pinned jar and `KRIT_NO_JAR_DOWNLOAD=1`, so a run
  never downloads.
- Pass `--fail-on-degraded`, so a scan whose FIR pass could not run exits 2
  instead of producing a Go-only result the build system would cache.
- Treat exit codes 0 (no findings) and 1 (findings) as success and 2 or more as
  failure.
