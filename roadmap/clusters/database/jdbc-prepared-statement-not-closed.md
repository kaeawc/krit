# JdbcPreparedStatementNotClosed

**Cluster:** [database](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Connection.prepareStatement(...)` not wrapped in `use { }` or
followed by explicit `.close()`.

## Triggers

```kotlin
val stmt = connection.prepareStatement("SELECT 1")
val rs = stmt.executeQuery()
```

## Does not trigger

```kotlin
connection.prepareStatement("SELECT 1").use { stmt ->
    stmt.executeQuery().use { rs -> /* ... */ }
}
```

## Dispatch

`call_expression` on `prepareStatement` whose result flows into a
`val`/`var` not used as a receiver of `use`/`.close()`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
