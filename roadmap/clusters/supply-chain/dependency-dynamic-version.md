# DependencyDynamicVersion

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`"group:name:1.+"`, `"group:name:+"`, or `".RELEASE"` / `latest`
wildcards in dependency coordinates.

## Triggers

```kotlin
implementation("androidx.core:core-ktx:1.+")
```

## Does not trigger

```kotlin
implementation("androidx.core:core-ktx:1.12.0")
```

## Configuration

No configuration. Active by default. Baseline legitimate exceptions.

## Dispatch

Extend the existing AOSP `GradleDynamicVersion` rule to KTS syntax.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
- Related: existing AOSP `GradleDynamicVersion`
