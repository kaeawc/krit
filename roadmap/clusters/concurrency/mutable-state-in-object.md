# MutableStateInObject

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`object Foo { var count = 0 }` — shared mutable state across all
threads without synchronization.

## Triggers

```kotlin
object Counter { var total = 0 }
```

## Does not trigger

```kotlin
object Counter { private val total = AtomicInteger(0) }
```

## Dispatch

`object_declaration` containing `var` properties with primitive /
collection types.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
- Related: `roadmap/clusters/testing-quality/shared-mutable-state-in-object.md`
