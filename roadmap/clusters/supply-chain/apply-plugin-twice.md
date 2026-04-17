# ApplyPluginTwice

**Cluster:** [supply-chain](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

Same plugin applied in both `plugins { }` and `apply(plugin = ...)`
in one build file.

## Triggers

```kotlin
plugins { id("com.android.application") }
apply(plugin = "com.android.application")
```

## Does not trigger

Either form, but not both.

## Dispatch

Walk both blocks; flag overlaps.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
