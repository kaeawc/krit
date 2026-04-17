# HardcodedEnvironmentName

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

String literal `"staging"`/`"dev"`/`"qa"`/`"prod"`/`"localhost"`
passed to a function whose name contains `Config`/`Env`/`Environment`.

## Triggers

```kotlin
val api = apiConfig("staging")
```

## Does not trigger

```kotlin
val api = apiConfig(BuildConfig.ENV)
```

## Dispatch

`call_expression` argument check gated on enclosing function name.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
