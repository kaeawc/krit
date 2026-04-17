# FlowWithoutFlowOn

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`Flow<T>` builder chain whose terminal operator does not have a
preceding `.flowOn(Dispatchers.IO|Default)` when the upstream
contains a blocking call.

## Triggers

```kotlin
flow {
    val rows = db.query() // blocking
    emit(rows)
}.collect { render(it) }
```

## Does not trigger

```kotlin
flow {
    val rows = db.query()
    emit(rows)
}.flowOn(Dispatchers.IO).collect { render(it) }
```

## Dispatch

`call_expression` chain ending in `collect`/`first`/`toList`;
walk upstream for blocking API calls and `flowOn`.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
