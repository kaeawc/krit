# MetricCounterNotMonotonic

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`counter.increment(-1.0)` — counters are monotonic by contract.

## Triggers

```kotlin
counter.increment(-1.0)
```

## Does not trigger

```kotlin
gauge.decrement()
```

## Dispatch

`call_expression` on `counter.increment` with a negative literal.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
