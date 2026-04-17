# ComposeLaunchedEffectWithoutKeys

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`LaunchedEffect(Unit) { ... }` where the body references a non-
`remember` variable — the effect won't re-run when the variable
changes.

## Triggers

```kotlin
LaunchedEffect(Unit) {
    fetch(userId)
}
```

## Does not trigger

```kotlin
LaunchedEffect(userId) {
    fetch(userId)
}
```

## Dispatch

`call_expression` on `LaunchedEffect(Unit)` / `LaunchedEffect(true)`
whose body references a variable not listed as a key.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
