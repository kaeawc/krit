# SynchronizedOnString

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`synchronized("lockName") { ... }` — interned strings share a
monitor across classloaders.

## Triggers

```kotlin
synchronized("global") { mutate() }
```

## Does not trigger

```kotlin
private val lock = Any()
synchronized(lock) { mutate() }
```

## Dispatch

`call_expression` on `synchronized` with a string-literal lock arg.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
