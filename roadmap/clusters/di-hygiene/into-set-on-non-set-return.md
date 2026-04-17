# IntoSetOnNonSetReturn

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@IntoSet @Provides fun` whose return type is not the set element
type. Dagger collects by return type, so a mismatch drops the
contribution silently.

## Triggers

```kotlin
@Provides @IntoSet
fun providePluginList(): List<Plugin> = listOf(...)
// intended to contribute to Set<Plugin>, actually contributes to Set<List<Plugin>>
```

## Does not trigger

```kotlin
@Provides @IntoSet
fun providePlugin(): Plugin = PluginImpl
```

## Dispatch

`@IntoSet` function whose return type is a collection wrapper.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
