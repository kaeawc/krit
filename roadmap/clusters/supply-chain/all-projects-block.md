# AllProjectsBlock

**Cluster:** [supply-chain](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`allprojects { }` block anywhere in the build. Deprecated in
Gradle 8.x; convention plugins are the replacement.

## Triggers

```kotlin
allprojects { repositories { mavenCentral() } }
```

## Does not trigger

```kotlin
// use settings-level repositories or a convention plugin
```

## Dispatch

`call_expression` on `allprojects`.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
