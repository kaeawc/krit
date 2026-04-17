# AnvilContributesBindingWithoutScope

**Cluster:** [di-hygiene](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`@ContributesBinding(SomeScope::class)` where the bound interface
is not visible in that scope's graph.

## Triggers

```kotlin
@ContributesBinding(AppScope::class)
class FeatureImpl @Inject constructor() : FeatureApi
// FeatureApi is only visible in FeatureScope
```

## Does not trigger

```kotlin
@ContributesBinding(AppScope::class)
class FeatureImpl @Inject constructor() : FeatureApi
// FeatureApi is an app-wide interface
```

## Dispatch

Cross-file reference check: resolve the bound interface and the
scope graph, flag if the interface is declared in a different
scope's module.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
