# JdbcResultSetLeakedFromFunction

**Cluster:** [database](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

Function whose return type is `ResultSet` — callers almost always
forget to close it.

## Triggers

```kotlin
fun query(sql: String): ResultSet = stmt.executeQuery(sql)
```

## Does not trigger

```kotlin
fun <R> query(sql: String, block: (ResultSet) -> R): R =
    stmt.executeQuery(sql).use(block)
```

## Dispatch

`function_declaration` whose declared return type resolves to
`java.sql.ResultSet`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
