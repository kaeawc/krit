# WithContextWithoutTracingContext

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`withContext(Dispatchers.IO) { ... }` inside a function that has an
active span — span disappears on the other dispatcher.

## Triggers

```kotlin
fun handle() {
    tracer.spanBuilder("handle").startSpan().use {
        runBlocking { withContext(Dispatchers.IO) { fetch() } }
    }
}
```

## Does not trigger

`withContext(Dispatchers.IO + otelContext.asContextElement()) { ... }`

## Dispatch

Enclosing-span detection: walk ancestors for `spanBuilder...startSpan`
and flag if `withContext` doesn't include the tracing element.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
