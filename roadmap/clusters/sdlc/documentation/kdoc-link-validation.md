# KdocLinkValidation

**Cluster:** [sdlc/documentation](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Concept

`[SomeClass]` references in KDoc resolved against the symbol index;
broken links flagged.

## Triggers

```kotlin
/**
 * Loads a user. See [NonExistentClass] for details.
 */
fun load(): User = ...
```

## Does not trigger

The KDoc link resolves to a real class.

## Dispatch

KDoc token scan + symbol index lookup.

## Links

- Parent: [`../README.md`](../README.md)
