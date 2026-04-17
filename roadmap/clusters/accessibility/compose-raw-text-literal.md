# ComposeRawTextLiteral

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Text("hardcoded string")` at top-level Compose function scope,
outside a file marked `@Preview` / `Sample`.

## Triggers

```kotlin
@Composable
fun Header() { Text("Welcome") }
```

## Does not trigger

```kotlin
@Composable
fun Header() { Text(stringResource(R.string.welcome)) }
```

## Dispatch

`call_expression` `Text(...)` inside a `@Composable`; skip if the
enclosing function is `@Preview` or its file is a sample.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
- Related: `roadmap/clusters/i18n/string-concat-for-translation.md`
