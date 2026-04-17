# PeriodicWorkRequestLessThan15Min

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`PeriodicWorkRequestBuilder(N, TimeUnit.MINUTES)` with `N < 15` —
minimum is 15 minutes; WorkManager silently upgrades.

## Triggers

```kotlin
PeriodicWorkRequestBuilder<Sync>(10, TimeUnit.MINUTES).build()
```

## Does not trigger

```kotlin
PeriodicWorkRequestBuilder<Sync>(15, TimeUnit.MINUTES).build()
```

## Dispatch

`call_expression` on `PeriodicWorkRequestBuilder` with literal
interval args.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
