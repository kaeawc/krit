# ComposeTextFieldMissingLabel

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`TextField(...)` / `OutlinedTextField(...)` with no `label` argument
and no accompanying `Text` sibling inside the same `Column`/`Row`.

## Triggers

```kotlin
TextField(value = email, onValueChange = { email = it })
```

## Does not trigger

```kotlin
TextField(
    value = email,
    onValueChange = { email = it },
    label = { Text(stringResource(R.string.email)) },
)
```

## Dispatch

`call_expression` on `TextField`/`OutlinedTextField` without `label`.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
