# LoggerStringConcat

**Cluster:** [observability](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`logger.info("value=" + value)` — same concern as
[`logger-interpolated-message.md`](logger-interpolated-message.md).

## Triggers

```kotlin
logger.info("value=" + value)
```

## Does not trigger

```kotlin
logger.info("value={}", value)
```

## Dispatch

`call_expression` on SLF4J/Logback logger with a `+` binary
expression as the message argument.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
