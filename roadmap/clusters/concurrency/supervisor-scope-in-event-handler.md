# SupervisorScopeInEventHandler

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`supervisorScope { ... }` wrapped around a single operation — the
supervisor semantics only matter for child divergence, so a
single-child use is overhead.

## Triggers

```kotlin
suspend fun handle() = supervisorScope { fetch() }
```

## Does not trigger

```kotlin
suspend fun handle() = supervisorScope {
    val a = async { fetchA() }
    val b = async { fetchB() }
    a.await() to b.await()
}
```

## Dispatch

`call_expression` on `supervisorScope` whose trailing lambda body
is a single statement.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
