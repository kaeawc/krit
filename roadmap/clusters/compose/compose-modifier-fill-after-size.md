# ComposeModifierFillAfterSize

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** info · **Default:** inactive

## Catches

`Modifier.size(48.dp).fillMaxWidth()` — the `fillMaxWidth` overrides
the explicit size on one axis; likely author mistake.

## Triggers

```kotlin
Modifier.size(48.dp).fillMaxWidth()
```

## Does not trigger

```kotlin
Modifier.fillMaxWidth().height(48.dp)
```

## Dispatch

Modifier chain walk for the `size → fillMax*` order.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
