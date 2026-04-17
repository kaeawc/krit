# ComposeMutableDefaultArgument

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`@Composable fun Foo(list: MutableList<X> = mutableListOf())` —
default evaluates each recomposition.

## Triggers

```kotlin
@Composable fun Foo(items: MutableList<String> = mutableListOf()) { ... }
```

## Does not trigger

```kotlin
@Composable fun Foo(items: List<String> = emptyList()) { ... }
```

## Dispatch

`function_declaration` annotated `@Composable` whose default args
include `mutableListOf`/`mutableSetOf`/`mutableMapOf`.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
