# PackageDependencyCycle

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Cycle in the package-level import graph within a single module.

## Triggers

`com.example.a.A` references `com.example.b.B`;
`com.example.b.B` references `com.example.a.A`.

## Does not trigger

Package imports form a DAG.

## Dispatch

Tarjan SCC over the per-package import graph derived from the
cross-file index.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`module-dependency-cycle.md`](module-dependency-cycle.md)
