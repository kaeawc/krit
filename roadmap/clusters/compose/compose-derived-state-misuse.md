# ComposeDerivedStateMisuse

**Cluster:** [compose](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`derivedStateOf { x > 0 }` where `x` is itself a `State<T>` — the
surrounding read already triggers recomposition; `derivedStateOf`
is only useful when the derived value changes less often than its
input.

## Triggers

```kotlin
val isPositive by remember { derivedStateOf { count > 0 } }
// where `count` is itself a State
```

## Does not trigger

```kotlin
// count is a State, but the derived read is coarser (grouping by tens)
val bucket by remember { derivedStateOf { count / 10 } }
```

## Dispatch

`call_expression` on `derivedStateOf` whose body is a boolean
comparison of a single `State<T>` access.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
