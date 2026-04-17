# DependenciesInRootProject

**Cluster:** [supply-chain](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`dependencies { }` block at the root `settings.gradle(.kts)` level.

## Triggers

```kotlin
// root build.gradle.kts
dependencies {
    implementation("com.example:lib:1.0")
}
```

## Does not trigger

Dependencies declared in per-module build files.

## Configuration

```yaml
supply-chain:
  DependenciesInRootProject:
    allowedConfigurations:
      - "classpath"
      - "detektPlugins"
```

`allowedConfigurations` lists dependency configurations that are
acceptable in the root project. Covers build-tooling dependencies
(buildscript classpath, test aggregation, lint plugin declarations)
that legitimately live at the root. `implementation`/`api`/`runtimeOnly`
in the root are still flagged.

## Dispatch

Gradle file check; flag if `dependencies { }` appears in the
project root.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
