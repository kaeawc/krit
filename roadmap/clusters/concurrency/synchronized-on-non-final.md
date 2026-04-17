# SynchronizedOnNonFinal

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`private var lock = Any()` used inside `synchronized(lock) { }` —
reassignment changes the monitor object.

## Triggers

```kotlin
private var lock = Any()
fun op() { synchronized(lock) { work() } }
```

## Does not trigger

```kotlin
private val lock = Any()
fun op() { synchronized(lock) { work() } }
```

## Dispatch

`call_expression` on `synchronized` whose argument name resolves to
a `var` property.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
