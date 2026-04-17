# RequiresOptInWithoutLevel

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Custom `@RequiresOptIn` annotation class without
`level = WARNING|ERROR`.

## Triggers

```kotlin
@RequiresOptIn annotation class InternalApi
```

## Does not trigger

```kotlin
@RequiresOptIn(level = RequiresOptIn.Level.ERROR)
annotation class InternalApi
```

## Dispatch

`class_declaration` of `annotation class` annotated `@RequiresOptIn`.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
