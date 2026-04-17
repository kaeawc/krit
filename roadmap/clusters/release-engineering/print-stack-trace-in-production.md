# PrintStackTraceInProduction

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`e.printStackTrace()` in a non-test file that imports a logging
framework.

## Triggers

```kotlin
import timber.log.Timber

try { load() } catch (e: Exception) { e.printStackTrace() }
```

## Does not trigger

```kotlin
import timber.log.Timber

try { load() } catch (e: Exception) { Timber.e(e, "load failed") }
```

## Dispatch

`call_expression` on `.printStackTrace()` in a file with a logging
import.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
- Related: `roadmap/clusters/security/call-shape/print-stack-trace-in-release.md`
