# ComposeDecorativeImageContentDescription

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Image(..., contentDescription = null)` without a `Modifier.semantics {
invisibleToUser() }` or `.clearAndSetSemantics { }` hint to TalkBack.

## Triggers

```kotlin
Image(painterResource(R.drawable.decoration), contentDescription = null)
```

## Does not trigger

```kotlin
Image(
    painterResource(R.drawable.decoration),
    contentDescription = null,
    modifier = Modifier.clearAndSetSemantics { },
)
```

## Dispatch

`call_expression` on `Image`/`AsyncImage` where
`contentDescription = null`; walk siblings/modifier chain for an
explicit hide-from-a11y hint.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
- Related: [`compose-icon-button-missing-content-description.md`](compose-icon-button-missing-content-description.md)
