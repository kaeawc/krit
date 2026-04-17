# ComposeIconButtonMissingContentDescription

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`IconButton`, `Icon`, `Image`, `AsyncImage` call without an explicit
`contentDescription` argument.

## Triggers

```kotlin
IconButton(onClick = { nav.pop() }) {
    Icon(Icons.Default.ArrowBack)
}
```

## Does not trigger

```kotlin
IconButton(onClick = { nav.pop() }) {
    Icon(Icons.Default.ArrowBack, contentDescription = stringResource(R.string.back))
}

// Decorative only:
Icon(Icons.Default.Star, contentDescription = null,
     modifier = Modifier.semantics { invisibleToUser() })
```

## Dispatch

`call_expression` on the listed composables where the named argument
`contentDescription` is absent.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
- Related: [`compose-decorative-image-content-description.md`](compose-decorative-image-content-description.md)
