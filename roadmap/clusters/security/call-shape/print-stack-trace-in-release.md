# PrintStackTraceInRelease

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`exception.printStackTrace()` in a file that does not import a logging
framework and is not in a test source set.

## Triggers

```kotlin
try {
    fetch()
} catch (e: IOException) {
    e.printStackTrace()
}
```

## Does not trigger

```kotlin
import timber.log.Timber

try { fetch() } catch (e: IOException) { Timber.e(e, "fetch failed") }
```

## Dispatch

`call_expression` on `.printStackTrace()` gated on file imports and
`isTestFile`.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: `roadmap/clusters/release-engineering/println-in-production.md`
