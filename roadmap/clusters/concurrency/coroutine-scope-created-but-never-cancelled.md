# CoroutineScopeCreatedButNeverCancelled

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`CoroutineScope(SupervisorJob())` stored in a property without a
corresponding `onCleared()` / `close()` call.

## Triggers

```kotlin
class ImageCache {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}
```

## Does not trigger

```kotlin
class ImageCache : Closeable {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    override fun close() { scope.cancel() }
}
```

## Dispatch

`property_declaration` whose RHS creates a `CoroutineScope`; walk
class body for `.cancel()` on the same variable.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
