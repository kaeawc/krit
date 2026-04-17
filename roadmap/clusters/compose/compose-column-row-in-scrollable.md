# ComposeColumnRowInScrollable

**Cluster:** [compose](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Column(Modifier.verticalScroll(...))` with an inner `LazyColumn` —
nested scroll containers with infinite constraints crash.

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

`call_expression` on `Column`/`Row` with `verticalScroll`/`horizontalScroll`
in the Modifier chain whose body contains a `LazyColumn`/`LazyRow`.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
- Related: `roadmap/clusters/resource-cost/lazy-column-in-side-column.md`
