# IntoMapMissingKey

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@IntoMap @Provides`/`@Binds` function with no `@*Key(...)` annotation.

## Triggers

```kotlin
@Provides @IntoMap
fun provideHandler(): Handler = HandlerImpl()
```

## Does not trigger

```kotlin
@Provides @IntoMap @StringKey("foo")
fun provideHandler(): Handler = HandlerImpl()
```

## Dispatch

`function_declaration` annotated `@IntoMap` without a sibling
`@*Key(...)` annotation.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
- Related: [`into-map-duplicate-key.md`](into-map-duplicate-key.md)
