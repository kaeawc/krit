# NonAsciiIdentifier

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Class/function/property name containing non-ASCII characters.
Portability risk on non-UTF-8 build environments and search/indexing.

## Triggers

```kotlin
class Résumé
```

## Does not trigger

```kotlin
class Resume
```

## Dispatch

Declaration-name scan.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
