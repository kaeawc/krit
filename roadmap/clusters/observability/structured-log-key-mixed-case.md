# StructuredLogKeyMixedCase

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`logger.atInfo().addKeyValue("userId", ...)` — project convention
is snake_case vs camelCase; flag the minority within one file.

## Triggers

```kotlin
logger.atInfo()
    .addKeyValue("user_id", id)
    .addKeyValue("requestId", req) // mixed convention
    .log("done")
```

## Does not trigger

Single consistent convention in the file.

## Dispatch

Scan `addKeyValue`/structured-field keys across a file.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
