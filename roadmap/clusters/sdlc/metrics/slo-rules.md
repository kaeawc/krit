# SloRules

**Cluster:** [sdlc/metrics](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Concept

"module X must have < N findings per 1000 LOC" — config-driven
aggregate thresholds that emit a finding if exceeded.

## Configuration

```yaml
slos:
  - module: ":core"
    max_warnings_per_kloc: 5
  - module: ":ui"
    max_warnings_per_kloc: 10
```

## Triggers

`:core` has 6.2 warnings / 1kLOC.

## Does not trigger

All modules within budget.

## Dispatch

Post-pass over the finding set, aggregated by module.

## Links

- Parent: [`../README.md`](../README.md)
