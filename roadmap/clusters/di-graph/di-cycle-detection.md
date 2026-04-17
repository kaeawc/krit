# DiCycleDetection

**Cluster:** [di-graph](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

Cycle in the DI binding graph — Dagger catches this at build time,
but points at generated code; krit can point at the source
declarations.

## Triggers

`A` injects `B`, `B` injects `C`, `C` injects `A`.

## Does not trigger

DAG.

## Dispatch

Tarjan SCC over the binding graph produced by
[`whole-graph-binding-completeness.md`](whole-graph-binding-completeness.md).

## Links

- Parent: [`../README.md`](../README.md)
