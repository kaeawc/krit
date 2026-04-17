# RoomSelectStarWithoutLimit

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Query("SELECT * FROM users")` without a `LIMIT` clause on a
non-`Flow`/non-`PagingSource` return type — unbounded load.

## Triggers

```kotlin
@Query("SELECT * FROM users") suspend fun all(): List<User>
```

## Does not trigger

```kotlin
@Query("SELECT * FROM users LIMIT 50") suspend fun all(): List<User>
@Query("SELECT * FROM users") fun observe(): Flow<List<User>>
@Query("SELECT * FROM users") fun paged(): PagingSource<Int, User>
```

## Dispatch

`function_declaration` annotated `@Query` whose argument contains
`SELECT *` without `LIMIT`; return type must not be Flow/Paging.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
