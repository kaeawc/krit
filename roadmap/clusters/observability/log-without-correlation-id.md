# LogWithoutCorrelationId

**Cluster:** [observability](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Logger call inside a coroutine scope whose context does not
include an MDC context element.

## Triggers

```kotlin
launch { logger.info("work started") }
```

## Does not trigger

```kotlin
launch(MDCContext()) { logger.info("work started") }
```

## Dispatch

Logger `call_expression` inside `launch`/`async`/`withContext` whose
context argument doesn't include `MDCContext`.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
