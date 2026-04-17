# BindsReturnTypeMatchesParam

**Cluster:** [di-hygiene](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`@Binds abstract fun bind(impl: Foo): Foo` — same type on both
sides is a no-op binding.

## Triggers

```kotlin
@Binds abstract fun bind(foo: Foo): Foo
```

## Does not trigger

```kotlin
@Binds abstract fun bind(impl: FooImpl): Foo
```

## Dispatch

`@Binds` function where parameter type text == return type text.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
