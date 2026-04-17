# MixedAssertionLibraries

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

A single test file imports both `org.junit.Assert.*` and
`com.google.common.truth.Truth.*`.

## Triggers

```kotlin
import org.junit.Assert.assertEquals
import com.google.common.truth.Truth.assertThat
```

## Does not trigger

One consistent import.

## Dispatch

Import-header scan.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
