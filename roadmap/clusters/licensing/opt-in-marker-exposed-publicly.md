# OptInMarkerExposedPublicly

**Cluster:** [licensing](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`@OptIn` on a `public` API — opt-in annotations propagate; callers
must opt in too.

## Triggers

```kotlin
@OptIn(ExperimentalCoroutinesApi::class)
public fun exposeExperimental(): SharedFlow<Int> = ...
```

## Does not trigger

```kotlin
@OptIn(ExperimentalCoroutinesApi::class)
private fun internalUse() = ...
```

## Dispatch

`annotation` on `@OptIn` whose target is `public`.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
