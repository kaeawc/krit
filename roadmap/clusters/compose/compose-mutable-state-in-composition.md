# ComposeMutableStateInComposition

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`val count = mutableStateOf(0)` as a local inside a `@Composable`
without `by remember { ... }` — creates a fresh state every
recomposition.

## Triggers

```kotlin
@Composable
fun Counter() {
    val count = mutableStateOf(0)
    Text(count.value.toString())
}
```

## Does not trigger

```kotlin
@Composable
fun Counter() {
    var count by remember { mutableStateOf(0) }
    Text(count.toString())
}
```

## Dispatch

`property_declaration` inside a `@Composable` whose RHS is
`mutableStateOf(...)` without an enclosing `remember { }`.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
