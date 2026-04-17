# BindsInsteadOfProvides

**Cluster:** [di-hygiene](README.md) · **Status:** in_progress · **Severity:** info · **Default:** active

## Catches

`@Provides fun provideFoo(impl: FooImpl): Foo = impl` — can be a
cheaper `@Binds abstract fun bindFoo(impl: FooImpl): Foo`.

## Triggers

```kotlin
@Module
object FooModule {
    @Provides fun provideFoo(impl: FooImpl): Foo = impl
}
```

## Does not trigger

```kotlin
@Module
abstract class FooModule {
    @Binds abstract fun bindFoo(impl: FooImpl): Foo
}
```

## Dispatch

`@Provides` function with a single parameter, a direct return, and
a body that returns the parameter unchanged.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
