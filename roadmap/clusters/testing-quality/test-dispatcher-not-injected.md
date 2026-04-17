# TestDispatcherNotInjected

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Test file references `Dispatchers.IO`/`Dispatchers.Default` directly
— should use `StandardTestDispatcher` / `UnconfinedTestDispatcher`.

## Triggers

```kotlin
@Test fun works() = runTest(Dispatchers.IO) { /* ... */ }
```

## Does not trigger

```kotlin
@Test fun works() = runTest(UnconfinedTestDispatcher()) { /* ... */ }
```

## Dispatch

Test-file scan for `Dispatchers.IO` / `Dispatchers.Default`.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
