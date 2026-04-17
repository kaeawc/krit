# DependencyFromJcenter

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`build.gradle(.kts)` contains `jcenter()` — JCenter sunset in 2021.

## Triggers

```kotlin
repositories {
    jcenter()
    mavenCentral()
}
```

## Does not trigger

```kotlin
repositories {
    mavenCentral()
    google()
}
```

## Dispatch

`call_expression` on `jcenter()` inside a `repositories` block.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
