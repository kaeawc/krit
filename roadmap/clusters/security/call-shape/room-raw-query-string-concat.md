# RoomRawQueryStringConcat

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`SimpleSQLiteQuery("...$x...")` or a `@RawQuery` DAO function
receiving a concatenated string.

## Triggers

```kotlin
@RawQuery
fun search(query: SimpleSQLiteQuery): List<User>

val q = SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE '%$term%'")
dao.search(q)
```

## Does not trigger

```kotlin
val q = SimpleSQLiteQuery(
    "SELECT * FROM users WHERE name LIKE ?",
    arrayOf("%$term%"),
)
dao.search(q)
```

## Dispatch

`call_expression` on `SimpleSQLiteQuery(...)` with the shared shape
helper. Distinct rule from `sql-injection-raw-query` because Room's
`@RawQuery` is a different API surface.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`sql-injection-raw-query.md`](sql-injection-raw-query.md),
  `roadmap/clusters/database/room-migration-uses-exec-sql-with-interpolation.md`
