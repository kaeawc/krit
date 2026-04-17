# LaunchWithoutCoroutineExceptionHandler

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`launch { ... }` whose body contains `throw` and whose enclosing
scope has no `CoroutineExceptionHandler` — GlobalScope / fire-and-
forget use cases.

## Triggers

```kotlin
GlobalScope.launch { throw RuntimeException("boom") }
```

## Does not trigger

```kotlin
GlobalScope.launch(CoroutineExceptionHandler { _, _ -> /*...*/ }) {
    throw RuntimeException("boom")
}
```

## Dispatch

`call_expression` on `launch` inside `GlobalScope`/unscoped context
whose body contains a `throw` expression.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
