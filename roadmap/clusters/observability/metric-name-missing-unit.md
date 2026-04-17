# MetricNameMissingUnit

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** info · **Default:** active

## Catches

Micrometer / Prometheus counter / gauge registered with a name that
doesn't end in a known unit suffix (`_total`, `_seconds`, `_bytes`,
`_count`).

## Triggers

```kotlin
registry.counter("requests")
```

## Does not trigger

```kotlin
registry.counter("requests_total")
```

## Dispatch

`call_expression` on the metrics registry counter/gauge/timer
constructors with a string-literal name.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
