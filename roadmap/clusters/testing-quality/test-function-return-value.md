# TestFunctionReturnValue

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@Test fun foo(): String = "..."` — JUnit discards the return
value; likely copy-paste bug.

## Triggers

```kotlin
@Test fun fingerprint(): String = "abc"
```

## Does not trigger

```kotlin
@Test fun fingerprint() { assertEquals("abc", compute()) }
// runTest suspending return is allowed:
@Test fun works() = runTest { /* ... */ }
```

## Dispatch

`@Test` function whose declared return type is neither `Unit` nor
the `TestResult` type from `kotlinx-coroutines-test`.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
