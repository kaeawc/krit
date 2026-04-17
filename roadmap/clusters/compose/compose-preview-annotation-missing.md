# ComposePreviewAnnotationMissing

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** info · **Default:** inactive

## Catches

`fun FooPreview()` whose body contains a `@Composable` call but
which itself is not annotated `@Preview`.

## Triggers

```kotlin
@Composable
fun FooPreview() { Foo() }
```

## Does not trigger

```kotlin
@Preview
@Composable
fun FooPreview() { Foo() }
```

## Dispatch

`function_declaration` whose name ends in `Preview` and whose body
calls a `@Composable` without the `@Preview` annotation.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
