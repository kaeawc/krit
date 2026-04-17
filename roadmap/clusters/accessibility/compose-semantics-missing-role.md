# ComposeSemanticsMissingRole

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Modifier.clickable` / `toggleable` / `selectable` without a later
`Modifier.semantics { role = ... }` or an explicit `role` named
argument on the modifier call.

## Triggers

```kotlin
Row(Modifier.clickable { toggle() }.padding(16.dp)) { /* ... */ }
```

## Does not trigger

```kotlin
Row(
    Modifier
        .clickable(role = Role.Button) { toggle() }
        .padding(16.dp),
) { /* ... */ }
```

## Dispatch

`call_expression` scan of the Modifier chain. Similar to
[`compose-clickable-without-min-touch-target.md`](compose-clickable-without-min-touch-target.md)
but inspecting the `role` argument instead.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
