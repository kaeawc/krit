# RoomReturnTypeFlowWithoutDistinct

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@Query(...) fun observe(): Flow<List<User>>` that can emit
redundant updates without a `.distinctUntilChanged()` at the
call-site.

## Triggers

```kotlin
val users = dao.observeUsers().collect { /* ... */ }
```

## Does not trigger

```kotlin
val users = dao.observeUsers().distinctUntilChanged().collect { /* ... */ }
```

## Dispatch

`@Query` DAO returning `Flow<...>`; then a reference-index lookup to
all callers, flagging ones missing `distinctUntilChanged`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
