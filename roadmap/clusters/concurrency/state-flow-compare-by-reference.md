# StateFlowCompareByReference

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`.map { it.field }.distinctUntilChanged()` — can be omitted when
the field is a primitive; StateFlow already dedupes by equality.

## Triggers

```kotlin
state.map { it.count }.distinctUntilChanged().collect { render(it) }
```

## Does not trigger

```kotlin
state.map { it.count }.collect { render(it) }
```

## Dispatch

`call_expression` chain matching `.map { ... }.distinctUntilChanged()`
where the map lambda returns a property read.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
