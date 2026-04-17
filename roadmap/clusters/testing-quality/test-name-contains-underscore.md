# TestNameContainsUnderscore

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`@Test fun foo_bar_baz()` when a sibling test file in the same
project uses backtick-quoted test names — convention mismatch.

## Triggers

```kotlin
@Test fun loads_user_when_cache_empty() { ... }
```

## Does not trigger

```kotlin
@Test fun `loads user when cache empty`() { ... }
```

## Dispatch

Test function-name scan; the convention is inferred per project
(majority-vote across the test source set).

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
