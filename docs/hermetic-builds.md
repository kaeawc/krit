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
