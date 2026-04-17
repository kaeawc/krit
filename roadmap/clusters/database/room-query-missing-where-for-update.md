# RoomQueryMissingWhereForUpdate

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Query("UPDATE ...")` / `DELETE` without a `WHERE` clause.

## Triggers

```kotlin
@Query("DELETE FROM users") suspend fun delete(): Int
```

## Does not trigger

```kotlin
@Query("DELETE FROM users WHERE id = :id") suspend fun delete(id: Long): Int
@Query("DELETE FROM users") suspend fun deleteAll(): Int // function name carveout
```

## Dispatch

`@Query` whose SQL text matches `UPDATE|DELETE` without `WHERE`,
unless the DAO function name starts with `deleteAll`/`clearAll`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
