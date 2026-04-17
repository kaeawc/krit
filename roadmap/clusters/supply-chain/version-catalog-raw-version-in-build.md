# VersionCatalogRawVersionInBuild

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`build.gradle.kts` contains a raw version literal for an artifact
that exists in the version catalog — should reference the alias.

## Triggers

```kotlin
implementation("com.squareup.okhttp3:okhttp:4.12.0")
// while libs.versions.toml has `okhttp = { module = ..., version = ... }`
```

## Does not trigger

```kotlin
implementation(libs.okhttp)
```

## Configuration

No configuration. Active by default. Baseline legitimate exceptions.

## Dispatch

Cross-file: TOML aliases resolved to coordinates, then detect raw
literal uses of those coordinates.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
