# KotlinVersionMismatchAcrossModules

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

Two modules declaring different `kotlin.jvmToolchain(...)` or
`kotlinOptions.jvmTarget`.

## Triggers

```kotlin
// :feature:a/build.gradle.kts
kotlin { jvmToolchain(17) }
// :feature:b/build.gradle.kts
kotlin { jvmToolchain(11) }
```

## Does not trigger

All modules agree on a single toolchain version.

## Configuration

```yaml
supply-chain:
  KotlinVersionMismatchAcrossModules:
    allowedMismatches:
      - modules: [":desktop", ":server"]
        reason: "JVM desktop targets require toolchain 21"
```

`allowedMismatches` lists module groups that intentionally differ.
Covers KMP projects where JVM, Android, and native targets have
different toolchain requirements by design.

## Dispatch

`BuildGraph` walk; compare per-module toolchain configs.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
