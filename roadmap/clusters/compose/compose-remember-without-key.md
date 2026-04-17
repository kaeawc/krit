# ComposeRememberWithoutKey

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`remember { expensiveBuilder(param) }` whose lambda references
`param` but `param` is not a key argument.

## Triggers

```kotlin
@Composable
fun Chart(dataset: List<Point>) {
    val series = remember { buildSeries(dataset) }
    Render(series)
}
```

## Does not trigger

```kotlin
@Composable
fun Chart(dataset: List<Point>) {
    val series = remember(dataset) { buildSeries(dataset) }
    Render(series)
}
```

## Dispatch

`call_expression` on `remember { ... }` whose lambda body references
identifiers that are not in the argument list of the `remember`
call itself.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
