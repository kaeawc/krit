# RunTestWithDelay

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`runTest { ... delay(N) ... }` where `N > 0` — should use
`advanceTimeBy(N)` for deterministic virtual time control.

## Triggers

```kotlin
@Test fun works() = runTest {
    delay(1000)
    assertThat(state.value).isEqualTo("ready")
}
```

## Does not trigger

```kotlin
@Test fun works() = runTest {
    advanceTimeBy(1000)
    assertThat(state.value).isEqualTo("ready")
}
// delay(0) carveout
@Test fun flush() = runTest { delay(0); /* ... */ }
```

## Dispatch

`call_expression` on `delay` inside a `runTest` lambda with a
non-zero argument.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
