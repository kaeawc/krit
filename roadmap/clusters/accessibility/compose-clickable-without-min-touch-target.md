# ComposeClickableWithoutMinTouchTarget

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Modifier.clickable { ... }` on a composable whose Modifier chain has
`.size(x)` / `.height(x)` / `.width(x)` with `x < 48.dp`.

## Triggers

```kotlin
Box(Modifier.size(32.dp).clickable { onTap() })
```

## Does not trigger

```kotlin
Box(Modifier.size(48.dp).clickable { onTap() })
Box(Modifier.minimumInteractiveComponentSize().clickable { onTap() })
```

## Dispatch

`call_expression` whose chain contains `Modifier.clickable`. Inspect
the same chain for `size/height/width` `.dp` literals.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
- Related: `roadmap/clusters/compose/compose-modifier-clickable-before-padding.md`
