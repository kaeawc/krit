# AssertTrueOnComparison

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** active

## Catches

`assertTrue(a == b)` / `assertTrue(a.isEqualTo(b))` — should be
`assertEquals(a, b)` for better failure messages.

## Triggers

```kotlin
assertTrue(actual == expected)
```

## Does not trigger

```kotlin
assertEquals(expected, actual)
```

## Dispatch

`call_expression` on `assertTrue` whose argument is a binary `==`
comparison.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
