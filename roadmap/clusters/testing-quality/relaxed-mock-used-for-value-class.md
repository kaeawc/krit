# RelaxedMockUsedForValueClass

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** active

## Catches

`mockk<Int>()` / `mockk<String>()` — mocking a primitive or value
class is almost always a mistake; use a literal.

## Triggers

```kotlin
val id = mockk<Long>(relaxed = true)
```

## Does not trigger

```kotlin
val id = 42L
```

## Dispatch

`call_expression` on `mockk<T>()` whose T is a primitive / String /
`@JvmInline value class`.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
