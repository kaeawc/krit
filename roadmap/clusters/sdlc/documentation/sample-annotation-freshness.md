# SampleAnnotationFreshness

**Cluster:** [sdlc/documentation](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Concept

`@Sample` tags reference functions; verify each referenced sample
still parses and the referenced symbol still exists.

## Triggers

```kotlin
/** @sample com.example.samples.removedSampleFn */
fun doc() { ... }
```

Where `removedSampleFn` no longer exists.

## Does not trigger

All `@sample` references resolve.

## Dispatch

KDoc tag scan + symbol index lookup.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`kdoc-link-validation.md`](kdoc-link-validation.md)
