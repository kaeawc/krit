# ModuleDependencyCycle

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Cross-module cycle in the dependency graph.

## Triggers

`:a` depends on `:b`, `:b` depends on `:c`, `:c` depends on `:a`.

## Does not trigger

The module graph is a DAG.

## Dispatch

Tarjan SCC over the module graph derived from every
`build.gradle(.kts)`.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`package-dependency-cycle.md`](package-dependency-cycle.md)
