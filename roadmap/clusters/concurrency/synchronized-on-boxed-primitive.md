# SynchronizedOnBoxedPrimitive

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`synchronized(1L)` or `synchronized(someInt)` where the receiver
resolves to a boxed primitive — identity-equality surprises.

## Triggers

```kotlin
val count: Int = 1
synchronized(count) { work() }
```

## Does not trigger

```kotlin
private val lock = Any()
synchronized(lock) { work() }
```

## Dispatch

`call_expression` on `synchronized` whose argument resolves to a
boxed `Int`/`Long`/`Boolean`.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
