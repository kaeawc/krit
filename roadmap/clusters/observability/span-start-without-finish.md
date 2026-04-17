# SpanStartWithoutFinish

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`tracer.spanBuilder(...).startSpan()` assigned to a local not
followed by `.end()` / `.use { ... }` / `.makeCurrent().use { ... }`.

## Triggers

```kotlin
val span = tracer.spanBuilder("work").startSpan()
doWork()
```

## Does not trigger

```kotlin
tracer.spanBuilder("work").startSpan().use { span ->
    doWork()
}
```

## Dispatch

`property_declaration` whose RHS ends in `.startSpan()`; check for
a matching `.end()` / `use` in the same block.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
