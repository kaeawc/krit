# MetricTimerOutsideBlock

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`timer.record { ... }` where the block is empty or a single
non-blocking call — no meaningful timing.

## Triggers

```kotlin
timer.record { field }
```

## Does not trigger

```kotlin
timer.record { expensiveIo() }
```

## Dispatch

`call_expression` on `timer.record` whose lambda body is a single
expression / property read.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
