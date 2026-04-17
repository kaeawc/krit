# DeferredAwaitInFinally

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`deferred.await()` inside a `finally { }` block — can throw during
cleanup and mask the original exception.

## Triggers

```kotlin
try {
    work()
} finally {
    cleanup.await()
}
```

## Does not trigger

```kotlin
try {
    work()
} finally {
    runCatching { cleanup.await() }
}
```

## Dispatch

`call_expression` on `.await()` nested inside a `finally_block`.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
