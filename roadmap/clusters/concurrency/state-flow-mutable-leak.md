# StateFlowMutableLeak

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`val _state = MutableStateFlow(...)` exposed publicly (`val state = _state`)
— breaks unidirectional data flow.

## Triggers

```kotlin
class VM {
    val state = MutableStateFlow(0) // public mutable
}
```

## Does not trigger

```kotlin
class VM {
    private val _state = MutableStateFlow(0)
    val state: StateFlow<Int> = _state
}
```

## Dispatch

`property_declaration` whose type is `MutableStateFlow<...>` and
visibility is public.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
