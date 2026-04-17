# ImageLoadedAtFullSizeInList

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Image loader call inside a `RecyclerView.ViewHolder` or
`LazyColumn { items { ... } }` without a size / override qualifier.

## Triggers

```kotlin
items(list) { item ->
    AsyncImage(model = item.url, contentDescription = null)
}
```

## Does not trigger

```kotlin
items(list) { item ->
    AsyncImage(
        model = item.url,
        contentDescription = null,
        modifier = Modifier.size(64.dp),
    )
}
```

## Dispatch

`call_expression` on image loader without a size/override arg in
a list-item context.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
