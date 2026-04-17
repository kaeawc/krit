# ComposePreviewWithBackingState

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`@Preview @Composable fun FooPreview()` whose body reads a real
ViewModel / Flow / runtime state holder.

## Triggers

```kotlin
@Preview
@Composable
fun FooPreview() {
    val vm: FooViewModel = hiltViewModel()
    Foo(vm.state.collectAsState().value)
}
```

## Does not trigger

```kotlin
@Preview
@Composable
fun FooPreview() {
    Foo(FakeFooState())
}
```

## Dispatch

`function_declaration` annotated `@Preview` whose body calls
`hiltViewModel()` / `viewModel()` / `collectAsState` / `LocalXXX.current`
on a real dependency.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
