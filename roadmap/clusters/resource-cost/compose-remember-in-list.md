# ComposeRememberInList

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`items { item -> remember { expensive() } }` without a key
argument — causes recomputation on list reordering.

## Triggers

```kotlin
LazyColumn {
    items(list) { item ->
        val state = remember { expensiveBuilder(item) }
    }
}
```

## Does not trigger

```kotlin
LazyColumn {
    items(list, key = { it.id }) { item ->
        val state = remember(item) { expensiveBuilder(item) }
    }
}
```

## Dispatch

`call_expression` on `remember` inside an `items { }` lambda
without an item-keyed argument list.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
- Related: `roadmap/clusters/compose/compose-remember-without-key.md`
