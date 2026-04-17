# HiltInstallInMismatch

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@InstallIn(SingletonComponent::class)` on a module that binds an
`@ActivityScoped` type — scope mismatch.

## Triggers

```kotlin
@Module
@InstallIn(SingletonComponent::class)
class FooModule {
    @Provides @ActivityScoped fun foo(): Foo = FooImpl()
}
```

## Does not trigger

```kotlin
@Module
@InstallIn(ActivityComponent::class)
class FooModule {
    @Provides @ActivityScoped fun foo(): Foo = FooImpl()
}
```

## Dispatch

`@Module` class annotated `@InstallIn`; walk providers for scope
annotations that don't match the component scope.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
