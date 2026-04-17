# ComposeStatefulDefaultParameter

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`@Composable fun Foo(state: MyState = MyState())` — default
`MyState()` allocates a fresh instance per recomposition, breaking
state hoisting and causing lost state.

## Triggers

```kotlin
@Composable fun Counter(state: CounterState = CounterState()) { ... }
```

## Does not trigger

```kotlin
@Composable
fun Counter(state: CounterState = rememberCounterState()) { ... }
```

## Dispatch

`function_declaration` annotated `@Composable` whose default
arguments include a constructor call.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
- Related: [`compose-mutable-default-argument.md`](compose-mutable-default-argument.md)
