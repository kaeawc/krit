# BindsMismatchedArity

**Cluster:** [di-hygiene](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@Binds` function with more than one parameter — Dagger rejects this
but the error is thrown at codegen time, not in the source view.

## Triggers

```kotlin
@Binds abstract fun bindFoo(a: FooImpl, b: BarImpl): Foo
```

## Does not trigger

```kotlin
@Binds abstract fun bindFoo(impl: FooImpl): Foo
```

## Dispatch

`function_declaration` annotated `@Binds` with parameter count ≠ 1.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
