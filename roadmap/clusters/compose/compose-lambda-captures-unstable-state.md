# ComposeLambdaCapturesUnstableState

**Cluster:** [compose](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Trailing lambda arg (`onClick = { vm.onEvent(item) }`) that captures
an unstable value without wrapping in `remember`.

## Triggers

```kotlin
LazyColumn {
    items(users) { user ->
        Button(onClick = { vm.select(user) }) { Text(user.name) }
    }
}
```

## Does not trigger

```kotlin
LazyColumn {
    items(users) { user ->
        val onClick = remember(user) { { vm.select(user) } }
        Button(onClick = onClick) { Text(user.name) }
    }
}
```

## Dispatch

`lambda_literal` passed as a `@Composable` parameter whose captured
variables include unstable types.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
