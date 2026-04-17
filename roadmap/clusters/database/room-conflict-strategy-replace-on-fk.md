# RoomConflictStrategyReplaceOnFk

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Insert(onConflict = REPLACE)` on an entity with foreign keys —
REPLACE deletes and re-inserts, cascading FK deletes.

## Triggers

```kotlin
@Insert(onConflict = OnConflictStrategy.REPLACE)
suspend fun insert(user: User)
// User has @ForeignKey(parent = Team::class)
```

## Does not trigger

```kotlin
@Insert(onConflict = OnConflictStrategy.IGNORE)
suspend fun insert(user: User)
```

## Dispatch

`@Insert` function targeting an entity that has `foreignKeys = [...]`
in the same project; cross-file index lookup.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
