# NullableStructuredField

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Structured log field whose value is a nullable expression without
a null fallback — emits `null` into log aggregation.

## Triggers

```kotlin
logger.atInfo().addKeyValue("user_id", user?.id).log("ready")
```

## Does not trigger

```kotlin
logger.atInfo().addKeyValue("user_id", user?.id ?: "anonymous").log("ready")
```

## Dispatch

`addKeyValue` argument is a `?.` chain without an elvis fallback.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
