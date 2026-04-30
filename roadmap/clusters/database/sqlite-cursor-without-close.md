# SqliteCursorWithoutClose

**Cluster:** [database](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`db.rawQuery(...)` / `db.query(...)` assigned to a `val` without
`.use { }`.

## Triggers

```kotlin
val cursor = db.rawQuery("SELECT * FROM users", null)
while (cursor.moveToNext()) { /* ... */ }
```

## Does not trigger

```kotlin
db.rawQuery("SELECT * FROM users", null).use { cursor ->
    while (cursor.moveToNext()) { /* ... */ }
}
```

## Dispatch

`property_declaration` whose RHS is a `rawQuery`/`query` call; no
surrounding `.use { ... }`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
