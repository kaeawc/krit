# OptInWithoutJustification

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@OptIn(SomeMarker::class)` on a declaration with no preceding KDoc
comment explaining why the opt-in is safe.

## Triggers

```kotlin
@OptIn(ExperimentalCoroutinesApi::class)
fun useApi() { /* ... */ }
```

## Does not trigger

```kotlin
/** Safe because the experimental flag is stable in our Kotlin version. */
@OptIn(ExperimentalCoroutinesApi::class)
fun useApi() { /* ... */ }
```

## Dispatch

`annotation` on `@OptIn`; walk preceding siblings for a `kdoc`
comment.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
