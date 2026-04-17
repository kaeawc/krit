# ConventionPluginAppliedToWrongTarget

**Cluster:** [supply-chain](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** inactive

## Catches

Convention plugin named `android-library` applied to a root
project or JVM-only module.

## Triggers

JVM module's build file applies `id("android-library")`.

## Does not trigger

Plugin applied to an actual Android module.

## Configuration

```yaml
supply-chain:
  ConventionPluginAppliedToWrongTarget:
    pluginTargetMap:
      "com.corp.android-module": "android"
      "com.corp.jvm-module": "jvm"
      "com.corp.kmp-module": "any"
```

`pluginTargetMap` maps custom convention plugin IDs to their
intended target kind (`android`, `jvm`, `any`). Without config the
rule infers from the plugin name (contains "android" → android
target). With config, inference is replaced by the explicit map.

## Dispatch

`BuildGraph` walk; cross-reference plugin ids with module kinds.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
