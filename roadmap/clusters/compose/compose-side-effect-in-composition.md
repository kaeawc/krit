# ComposeSideEffectInComposition

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

Direct mutable-property write inside a `@Composable` body, not
wrapped in `LaunchedEffect` / `SideEffect` / `DisposableEffect`.

## Triggers

```kotlin
@Composable
fun Screen(vm: VM) {
    vm.tracker.seen = true // side effect during composition
    Content(vm.state)
}
```

## Does not trigger

```kotlin
@Composable
fun Screen(vm: VM) {
    LaunchedEffect(Unit) { vm.tracker.seen = true }
    Content(vm.state)
}
```

## Dispatch

Assignment/call to setter inside a `@Composable` body not nested in
a recognized effect block.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
