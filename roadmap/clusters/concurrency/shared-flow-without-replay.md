# SharedFlowWithoutReplay

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`MutableSharedFlow<T>()` used as an event channel without explicit
`replay`/`extraBufferCapacity` — default is lossy.

## Triggers

```kotlin
private val events = MutableSharedFlow<Event>()
```

## Does not trigger

```kotlin
private val events = MutableSharedFlow<Event>(extraBufferCapacity = 1)
```

## Dispatch

`property_declaration` whose RHS is `MutableSharedFlow()` with no args.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
