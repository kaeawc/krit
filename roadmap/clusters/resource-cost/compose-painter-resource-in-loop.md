# ComposePainterResourceInLoop

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`painterResource(...)` inside a `forEach { }` or `items { }` block
— creates a fresh painter per iteration.

## Triggers

```kotlin
LazyColumn {
    items(list) { item ->
        Icon(painterResource(R.drawable.marker), null)
    }
}
```

## Does not trigger

```kotlin
val marker = painterResource(R.drawable.marker)
LazyColumn {
    items(list) { item -> Icon(marker, null) }
}
```

## Dispatch

`call_expression` on `painterResource` inside a `forEach`/`items`
lambda body.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
