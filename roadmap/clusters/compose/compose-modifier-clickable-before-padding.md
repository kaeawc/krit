# ComposeModifierClickableBeforePadding

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** inactive

## Catches

`Modifier.clickable { ... }.padding(16.dp)` — click area excludes
the padding region. Order in Compose modifiers matters.

## Triggers

```kotlin
Box(Modifier.clickable { onTap() }.padding(16.dp)) { Icon(...) }
```

## Does not trigger

```kotlin
Box(Modifier.padding(16.dp).clickable { onTap() }) { Icon(...) }
```

## Dispatch

`call_expression` chain walk detecting the `clickable → padding` order.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
