# VersionCatalogBuildSrcMismatch

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`buildSrc/src/main/kotlin/Versions.kt` constant and a
`libs.versions.toml` alias referencing the same artifact with
different versions.

## Triggers

```kotlin
// buildSrc/.../Versions.kt
object Versions { const val OKHTTP = "4.11.0" }
```
```toml
# gradle/libs.versions.toml
okhttp = "4.12.0"
```

## Does not trigger

Both sources agree, or only one exists.

## Dispatch

Cross-file: parse TOML, scan `buildSrc` Kotlin for matching
constant names, compare.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
