# TraceIdLoggedAsPlainMessage

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`logger.info("trace=$traceId $message")` — should be an MDC key /
structured field, not interpolation.

## Triggers

```kotlin
logger.info("trace=$traceId processed")
```

## Does not trigger

```kotlin
MDC.put("trace_id", traceId)
logger.info("processed")
```

## Dispatch

Logger call whose message template interpolates a variable with
`trace`/`traceId` identifier.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
