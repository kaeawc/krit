# RequiresOptInWithoutMessage

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@RequiresOptIn` without a `message = "..."` argument.

## Triggers

```kotlin
@RequiresOptIn annotation class InternalApi
```

## Does not trigger

```kotlin
@RequiresOptIn(message = "Internal API — subject to change.")
annotation class InternalApi
```

## Dispatch

Annotation argument check.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
