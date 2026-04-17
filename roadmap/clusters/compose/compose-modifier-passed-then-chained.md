# ComposeModifierPassedThenChained

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

Function parameter `modifier: Modifier = Modifier`, but the body
invokes an inner composable with a fresh `Modifier.X()` that drops
the passed-in `modifier`.

## Triggers

```kotlin
@Composable
fun Card(modifier: Modifier = Modifier) {
    Box(Modifier.fillMaxSize()) { /* caller modifier ignored */ }
}
```

## Does not trigger

```kotlin
@Composable
fun Card(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize()) { /* ... */ }
}
```

## Dispatch

`function_declaration` with `modifier: Modifier` parameter; check
that the inner composable call receives it.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
