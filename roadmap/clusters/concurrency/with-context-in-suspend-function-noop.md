# WithContextInSuspendFunctionNoop

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`withContext(Dispatchers.IO) { ... }` inside a function whose
parent is already a `withContext(Dispatchers.IO)` — redundant
context switch.

## Triggers

```kotlin
suspend fun outer() = withContext(Dispatchers.IO) {
    withContext(Dispatchers.IO) { work() }
}
```

## Does not trigger

```kotlin
suspend fun outer() = withContext(Dispatchers.IO) { work() }
```

## Dispatch

Walk ancestors of a `withContext` call looking for another
`withContext` on the same dispatcher.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
