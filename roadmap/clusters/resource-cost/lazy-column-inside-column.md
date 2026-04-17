# LazyColumnInsideColumn

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Compose: `LazyColumn` as a direct child of
`Column(Modifier.verticalScroll(...))` — unbounded scroll parent.

## Triggers

```kotlin
Column(Modifier.verticalScroll(rememberScrollState())) {
    Header()
    LazyColumn { items(list) { Row(it) } }
}
```

## Does not trigger

```kotlin
LazyColumn {
    item { Header() }
    items(list) { Row(it) }
}
```

## Dispatch

`call_expression` on `Column(...verticalScroll...)` whose body
contains a `LazyColumn` direct child.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
- Related: `roadmap/clusters/compose/compose-column-row-in-scrollable.md`
