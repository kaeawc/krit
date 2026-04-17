# LoggerWithoutLoggerField

**Cluster:** [observability](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`LoggerFactory.getLogger(...)` inside a function body rather than
at class-level — creates a new logger per call.

## Triggers

```kotlin
fun handle() {
    val log = LoggerFactory.getLogger(javaClass)
    log.info("handle")
}
```

## Does not trigger

```kotlin
private val log = LoggerFactory.getLogger(javaClass)
fun handle() { log.info("handle") }
```

## Dispatch

`call_expression` on `LoggerFactory.getLogger` inside a
`function_declaration` (not in a property initializer).

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
