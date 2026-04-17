# ScopeOnParameterizedClass

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Singleton` / `@ActivityScoped` on a generic class — scopes hold a
single instance, so the type parameter is erased at runtime.

## Triggers

```kotlin
@Singleton class Cache<K, V> @Inject constructor() { ... }
```

## Does not trigger

```kotlin
class Cache<K, V> @Inject constructor() { ... } // unscoped
```

## Dispatch

`class_declaration` with both a type_parameters child and a scope
annotation.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
