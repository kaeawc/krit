# MissingJvmSuppressWildcards

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Provides`/`@Binds` returning `Set<Foo>` / `Map<K, Foo>` in a
Kotlin module consumed by Dagger — needs `@JvmSuppressWildcards`
on the element type.

## Triggers

```kotlin
@Provides
fun providePlugins(): Set<Plugin> = setOf(...)
```

## Does not trigger

```kotlin
@Provides
fun providePlugins(): Set<@JvmSuppressWildcards Plugin> = setOf(...)
```

## Dispatch

`@Provides`/`@Binds` whose return type is a `Set<T>`/`Map<K, V>`
without an annotation on the element type.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
