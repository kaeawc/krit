# HiltSingletonWithActivityDep

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Singleton` class whose constructor takes `Activity`, `Fragment`,
`View`, or `LifecycleOwner` — scope mismatch.

## Triggers

```kotlin
@Singleton
class NavigatorImpl @Inject constructor(val activity: Activity) : Navigator
```

## Does not trigger

```kotlin
@ActivityScoped
class NavigatorImpl @Inject constructor(val activity: Activity) : Navigator
```

## Dispatch

`class_declaration` annotated `@Singleton` whose constructor args
include Activity-scoped types.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
