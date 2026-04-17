# OptInMarkerNotRecognised

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@OptIn(Foo::class)` where `Foo` is not in the embedded
well-known-markers list — likely a stale reference.

## Triggers

```kotlin
@OptIn(RemovedExperimentalApi::class)
fun f() { ... }
```

## Does not trigger

```kotlin
@OptIn(ExperimentalCoroutinesApi::class)
fun f() { ... }
```

## Dispatch

Annotation argument check against embedded list.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
