# JdbcStatementExecute

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`Statement.executeQuery(...)` / `.execute(...)` / `.executeUpdate(...)`
with an interpolated or concatenated string argument. Should be
`PreparedStatement` with bind parameters.

## Triggers

```kotlin
connection.createStatement().executeQuery("SELECT * FROM users WHERE id = $id")
```

## Does not trigger

```kotlin
connection.prepareStatement("SELECT * FROM users WHERE id = ?").use {
    it.setInt(1, id)
    it.executeQuery()
}
```

## Dispatch

`call_expression` on `executeQuery`/`execute`/`executeUpdate` where
the receiver resolves to `java.sql.Statement`. Shape helper reused.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`sql-injection-raw-query.md`](sql-injection-raw-query.md)
