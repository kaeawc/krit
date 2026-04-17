# WorkManagerNoBackoff

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`OneTimeWorkRequestBuilder<...>()` chain without `.setBackoffCriteria(...)`
on a worker whose class name suggests retry is meaningful.

## Triggers

```kotlin
val req = OneTimeWorkRequestBuilder<SyncWorker>().build()
```

## Does not trigger

```kotlin
val req = OneTimeWorkRequestBuilder<SyncWorker>()
    .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 10, TimeUnit.SECONDS)
    .build()
```

## Dispatch

`call_expression` chain on `OneTimeWorkRequestBuilder` whose
receiver class name ends in `SyncWorker`/`UploadWorker`/etc.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
