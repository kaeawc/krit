# BuildConfigDebugInverted

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`if (!BuildConfig.DEBUG) { logging }` — likely inverted guard.

## Triggers

```kotlin
if (!BuildConfig.DEBUG) {
    Log.d("TAG", "state=$state")
}
```

## Does not trigger

```kotlin
if (BuildConfig.DEBUG) {
    Log.d("TAG", "state=$state")
}
```

## Dispatch

`if_expression` with negated `BuildConfig.DEBUG` condition whose
body contains logging calls.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
