# WorkManagerUniquePolicyKeepButReplaceIntended

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`enqueueUniqueWork(name, KEEP, ...)` followed by application
restart logic — `REPLACE` may be intended.

## Triggers

```kotlin
workManager.enqueueUniqueWork("sync", ExistingWorkPolicy.KEEP, req)
// file also contains startRestartableServices() elsewhere
```

## Does not trigger

`ExistingWorkPolicy.REPLACE` explicitly.

## Dispatch

`call_expression` on `enqueueUniqueWork` with `KEEP` plus a
heuristic file-level check for restart logic.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
