# ConfigurationsAllSideEffect

**Cluster:** [supply-chain](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** inactive

## Catches

`configurations.all { ... }` block that mutates the dependency
graph — fragile, runs per configuration.

## Triggers

```kotlin
configurations.all {
    resolutionStrategy.force("com.squareup.okhttp3:okhttp:4.12.0")
}
```

## Does not trigger

```kotlin
configurations.matching { it.name == "runtimeClasspath" }.all { /* ... */ }
```

## Configuration

```yaml
supply-chain:
  ConfigurationsAllSideEffect:
    allowInConventionPlugins: true
```

`allowInConventionPlugins` (default true) skips the rule in files
under `build-logic/` or `buildSrc/`. Convention plugins are expected
to manipulate configurations — that's their job. Only flag
`configurations.all` in application/library `build.gradle.kts`.

## Dispatch

`call_expression` on `configurations.all`.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
