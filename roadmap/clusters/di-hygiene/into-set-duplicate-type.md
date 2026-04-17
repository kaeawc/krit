# IntoSetDuplicateType

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Two `@IntoSet` providers in the same module returning the same
concrete type — the set dedupes, so one contribution is lost.

## Triggers

```kotlin
@Provides @IntoSet fun provideA(): Plugin = PluginImpl()
@Provides @IntoSet fun provideB(): Plugin = PluginImpl()
```

## Does not trigger

```kotlin
@Provides @IntoSet fun provideA(): Plugin = PluginA()
@Provides @IntoSet fun provideB(): Plugin = PluginB()
```

## Dispatch

Cross-file `@IntoSet` aggregation, same index as
[`into-map-duplicate-key.md`](into-map-duplicate-key.md).

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
