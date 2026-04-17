# VisibleForTestingCallerInNonTest

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@VisibleForTesting` function called from a non-test file.

## Triggers

```kotlin
// src/main/java/.../ProdCaller.kt
fun run() { Helper.forTestingOnly() }
```

## Does not trigger

Call-site is inside a test file.

## Dispatch

Cross-file reference walk: find all `@VisibleForTesting` declarations,
enumerate their call-sites via the bloom filter index, flag
non-test callers.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
