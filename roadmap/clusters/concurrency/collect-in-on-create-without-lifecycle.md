# CollectInOnCreateWithoutLifecycle

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`flow.collect { }` inside `onCreate`/`onStart`/`onViewCreated`
without `repeatOnLifecycle` wrapping — collects while stopped.

## Triggers

```kotlin
override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)
    lifecycleScope.launch { vm.state.collect { render(it) } }
}
```

## Does not trigger

```kotlin
override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)
    lifecycleScope.launch {
        repeatOnLifecycle(Lifecycle.State.STARTED) {
            vm.state.collect { render(it) }
        }
    }
}
```

## Dispatch

`call_expression` on `.collect` inside known lifecycle callbacks.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
