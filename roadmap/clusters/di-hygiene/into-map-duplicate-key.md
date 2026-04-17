# IntoMapDuplicateKey

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

Cross-file: two `@IntoMap` providers with the same key value in the
same module/component — the second silently wins.

## Triggers

```kotlin
// FileA.kt
@Provides @IntoMap @StringKey("foo")
fun provideA(): Handler = HandlerA()

// FileB.kt
@Provides @IntoMap @StringKey("foo")
fun provideB(): Handler = HandlerB()
```

## Does not trigger

Two providers with distinct keys.

## Dispatch

Cross-file aggregation rule — uses the declaration index to group
`@IntoMap` providers by containing module/component and scan for
duplicate key literals.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
