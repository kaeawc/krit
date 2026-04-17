# CompileSdkMismatchAcrossModules

**Cluster:** [supply-chain](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Two Android modules declaring different `compileSdk`. Merged build
picks the max, silently changing behavior.

## Triggers

`:feature:a` has `compileSdk = 33`, `:feature:b` has `compileSdk = 34`.

## Does not trigger

All Android modules agree.

## Configuration

```yaml
supply-chain:
  CompileSdkMismatchAcrossModules:
    minimumSdk: 35
    allowedMismatches:
      - modules: [":wear"]
        reason: "Wear OS module targets compileSdk 33"
```

`minimumSdk` (default 35 for AGP 9) flags any module below this as
an error regardless of cross-module consistency. `allowedMismatches`
lists modules that intentionally differ from the majority.

## Dispatch

`BuildGraph` walk comparing `android { compileSdk = N }` across
modules.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
