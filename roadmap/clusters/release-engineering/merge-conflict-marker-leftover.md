# MergeConflictMarkerLeftover

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

File contains `<<<<<<<`, `=======`, `>>>>>>>` — unresolved merge
conflict.

## Triggers

```kotlin
<<<<<<< HEAD
val x = 1
=======
val x = 2
>>>>>>> feature
```

## Does not trigger

Any file without those markers.

## Dispatch

Line rule; cheap pre-commit-hook candidate.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
