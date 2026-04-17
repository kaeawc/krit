# LogLevelGuardMissing

**Cluster:** [observability](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`logger.debug("heavy-${serialize(thing)}")` where the interpolated
expression is a call — arguments are evaluated even when the level
is disabled.

## Triggers

```kotlin
logger.debug("payload=${serialize(thing)}")
```

## Does not trigger

```kotlin
if (logger.isDebugEnabled) {
    logger.debug("payload={}", serialize(thing))
}
```

## Dispatch

Logger call at `debug`/`trace` level whose message template
contains a function call interpolation.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
