# GlobalScopeLaunchInViewModel

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`GlobalScope.launch { ... }` inside a class whose name ends in
`ViewModel` or `Presenter` — should be `viewModelScope` / lifecycle
scope.

## Triggers

```kotlin
class UserViewModel : ViewModel() {
    fun load() { GlobalScope.launch { /* ... */ } }
}
```

## Does not trigger

```kotlin
class UserViewModel : ViewModel() {
    fun load() { viewModelScope.launch { /* ... */ } }
}
```

## Dispatch

`call_expression` on `GlobalScope.launch` inside a class with the
name suffix.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
