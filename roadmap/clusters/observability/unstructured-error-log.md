# UnstructuredErrorLog

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`logger.error("failure: $e")` — should be `logger.error("failure", e)`
so the stacktrace is a structured field.

## Triggers

```kotlin
logger.error("failure: $e")
```

## Does not trigger

```kotlin
logger.error("failure", e)
```

## Dispatch

`call_expression` on `logger.error` whose message template
interpolates a variable typed `Throwable`.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
