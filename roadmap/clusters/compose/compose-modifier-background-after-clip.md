# ComposeModifierBackgroundAfterClip

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** inactive

## Catches

`Modifier.background(Color.Red).clip(RoundedCornerShape(8.dp))` —
the background is drawn in the rectangular region, not the clipped
region.

## Triggers

```kotlin
Modifier.background(Color.Red).clip(RoundedCornerShape(8.dp))
```

## Does not trigger

```kotlin
Modifier.clip(RoundedCornerShape(8.dp)).background(Color.Red)
```

## Dispatch

Modifier chain walk for the `background → clip` order.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
