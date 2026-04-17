# VersionCatalogUnused

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

An alias declared in `libs.versions.toml` with zero references
across `build.gradle(.kts)` files in the project.

## Triggers

`libs.versions.toml` declares `okhttp = "4.12.0"` and
`okhttp = { module = "com.squareup.okhttp3:okhttp", version.ref = "okhttp" }`
but no `libs.okhttp` reference anywhere.

## Does not trigger

Alias referenced in at least one build file.

## Configuration

```yaml
supply-chain:
  VersionCatalogUnused:
    ignoredAliases:
      - "kotlin-bom"
      - "convention-*"
    scanConventionPlugins: true
```

`ignoredAliases` skips specific aliases (supports wildcards). Covers
entries consumed by convention plugins, BOMs applied at settings level,
or aliases reserved for future use. `scanConventionPlugins` (default
true) tells the rule to also search `build-logic/` source files for
catalog accessor references, not just module `build.gradle.kts` files.

## Dispatch

Version-catalog TOML parser + cross-file reference scan via the
module index.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
