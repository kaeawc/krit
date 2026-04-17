# MdcAcrossCoroutineBoundary

**Cluster:** [observability](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`MDC.put(...)` followed by a `launch { ... }` or `withContext { ... }`
— MDC does not propagate across dispatchers without `MDCContext`.

## Triggers

```kotlin
MDC.put("reqId", id)
launch { handle() }
```

## Does not trigger

```kotlin
MDC.put("reqId", id)
launch(MDCContext()) { handle() }
```

## Dispatch

`call_expression` on `MDC.put` with a subsequent sibling
`launch`/`withContext` lacking `MDCContext` in the context arg.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
