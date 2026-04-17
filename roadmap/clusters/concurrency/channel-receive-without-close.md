# ChannelReceiveWithoutClose

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Channel<T>()` declared at class scope, sent to, never closed —
leaks the receiver.

## Triggers

```kotlin
class Worker {
    private val events = Channel<Event>()
    fun send(e: Event) { events.trySend(e) }
}
```

## Does not trigger

```kotlin
class Worker : Closeable {
    private val events = Channel<Event>()
    fun send(e: Event) { events.trySend(e) }
    override fun close() { events.close() }
}
```

## Dispatch

`property_declaration` whose RHS is `Channel<T>()`; walk the class
body for a matching `.close()` call.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
