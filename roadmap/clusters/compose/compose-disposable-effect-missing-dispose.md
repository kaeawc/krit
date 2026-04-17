# ComposeDisposableEffectMissingDispose

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`DisposableEffect(key) { ... }` whose last statement is not
`onDispose { ... }`.

## Triggers

```kotlin
DisposableEffect(listener) {
    source.addListener(listener)
    // missing onDispose
}
```

## Does not trigger

```kotlin
DisposableEffect(listener) {
    source.addListener(listener)
    onDispose { source.removeListener(listener) }
}
```

## Dispatch

`call_expression` on `DisposableEffect` whose trailing lambda's
last statement isn't an `onDispose(...)` call.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
