# Full String Interning

**Cluster:** [performance-infra](./README.md) · **Status:** planned ·
**Supersedes:** part of [`roadmap/27-performance-algorithms.md`](../../27-performance-algorithms.md)

## What it is

Expand the current partial string interning (rule names, node types) to cover
all hot-path strings: file paths, qualified names, type signatures,
finding messages. Reduces GC pressure and memory usage on large repos.

## Current state

Partial interning exists in `internal/scanner/` for node type strings.
Item 27 landed several related optimizations: precomputed ancestor set,
supertype-DFS memoization, uint64-packed keys, atomic counters.

## Implementation notes

- Primary target: `internal/typeinfer/` qualified names, `internal/scanner/`
  file paths, `internal/rules/` finding message templates
- Status: exploratory — needs profiling to confirm memory is the bottleneck
  vs CPU on large repos
