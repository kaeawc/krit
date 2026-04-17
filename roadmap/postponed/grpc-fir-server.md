---
name: gRPC FIR server
status: postponed
---

# gRPC FIR server

## Status: postponed 2026-04-15

Considered during the performance-infra optimization push after
Strategy 1 (process sharding) hit a hardware ceiling around 1.4×
on the kotlin/kotlin repo. The idea was to run one dedicated
"FIR server" process holding the Analysis API cache, with N
stateless query clients sending type requests over gRPC.

## Why postponed

The design collapses to the existing `krit-types` daemon path:

- A single FIR server is **single-threaded** because
  Kotlin Analysis API is not thread-safe at the app level
  (`KotlinCoreEnvironment.ourApplicationEnvironment` is a JVM-wide
  singleton; see [KT-64167](https://youtrack.jetbrains.com/issue/KT-64167)).
- Multiple clients querying one server would serialize on the
  server's single-threaded analyze loop.
- That's functionally identical to the existing persistent
  daemon in `internal/oracle/daemon.go` and `cmd/krit-types`
  with `--daemon --port 0`.

Adding a gRPC transport and a client protocol would duplicate
the existing daemon's socket protocol without unlocking new
parallelism, at the cost of a new external dependency and
two language bindings.

## Revisit when

- Kotlin Analysis API documents a thread-safety contract that
  permits multiple concurrent `analyze {}` blocks on a single
  shared `KaSourceModule`. Currently they explicitly do not —
  the `KaSession` KDoc requires single-read-action confinement.
- Or: the daemon evolves to serve multiple concurrent krit
  processes on the same machine (different repos, shared JVM).
  That is a valid extension of item 21, not a separate track.

## Related

- [21-daemon-startup-optimization.md](../21-daemon-startup-optimization.md)
- Investigation notes in the
  `wt/parallel-jobs` (commit `ee3aef4`) and
  `wt/strategy3-warmup` (commit `1877929`) branches
