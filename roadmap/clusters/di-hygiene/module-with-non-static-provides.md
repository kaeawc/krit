# ModuleWithNonStaticProvides

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@Module abstract class` with a mix of `@Provides` and `@Binds`.
The `@Provides` methods should live in a companion `@Module object`.

## Triggers

```kotlin
@Module
abstract class FooModule {
    @Binds abstract fun bindA(impl: AImpl): A
    @Provides fun provideB(): B = BImpl()
}
```

## Does not trigger

```kotlin
@Module
abstract class FooModule {
    @Binds abstract fun bindA(impl: AImpl): A

    companion object {
        @Provides fun provideB(): B = BImpl()
    }
}
```

## Dispatch

`class_declaration` annotated `@Module` with `abstract` modifier;
walk class body for both `@Binds` and `@Provides` at top level.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
