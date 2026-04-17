# AssertEqualsArgumentOrder

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`assertEquals(actual, expected)` where the variable named `actual`
is the first argument. JUnit's convention is `(expected, actual)`.

## Triggers

```kotlin
val actual = compute()
val expected = 42
assertEquals(actual, expected)
```

## Does not trigger

```kotlin
assertEquals(expected, actual)
```

## Dispatch

`call_expression` on `assertEquals` inspecting argument variable
names.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
