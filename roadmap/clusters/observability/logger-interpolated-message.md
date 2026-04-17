# LoggerInterpolatedMessage

**Cluster:** [observability](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

SLF4J / Logback call like `logger.info("user $id logged in")`.
Should use the parameterised form so the template caches and the
call skips arg evaluation when the level is disabled.

## Triggers

```kotlin
logger.info("user $id logged in")
```

## Does not trigger

```kotlin
logger.info("user {} logged in", id)
```

## Dispatch

`call_expression` on SLF4J/Logback logger methods (from the
observability registry) with a string-template argument containing
interpolations. **Timber** is carve-out — its API is designed for
interpolation.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
