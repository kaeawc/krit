# VersionCatalogDuplicateVersion

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

Two aliases in `libs.versions.toml` pointing at different versions
of the same group:artifact.

## Triggers

```toml
[libraries]
okhttp-core = { module = "com.squareup.okhttp3:okhttp", version = "4.12.0" }
okhttp-alt  = { module = "com.squareup.okhttp3:okhttp", version = "4.11.0" }
```

## Does not trigger

```toml
okhttp = { module = "com.squareup.okhttp3:okhttp", version.ref = "okhttp" }
```

## Dispatch

TOML parser; detect duplicate `module` coordinates with distinct
versions.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
