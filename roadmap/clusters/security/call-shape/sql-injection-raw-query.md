# SqlInjectionRawQuery

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`db.rawQuery(...)` / `db.execSQL(...)` / `db.query(...)` whose SQL
string argument is an interpolated template or a `+`-concatenation
with a non-literal operand.

## Triggers

```kotlin
fun findById(db: SQLiteDatabase, id: String) {
    db.rawQuery("SELECT * FROM users WHERE id = $id", null)
}
```

## Does not trigger

```kotlin
fun findById(db: SQLiteDatabase, id: String) {
    db.rawQuery("SELECT * FROM users WHERE id = ?", arrayOf(id))
}

// Allowlisted: schema constants
db.rawQuery("SELECT * FROM ${Users.TABLE_NAME}", null)
```

## Dispatch

`call_expression` on `rawQuery` / `execSQL` / `query`; uses the shared
`argumentIsUntrustedShape(node, file)` helper returning
`Static`/`Interpolated`/`Computed`. ALL_CAPS or `*_TABLE`/`*_COLUMN`
identifiers get an allowlist carveout.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`room-raw-query-string-concat.md`](room-raw-query-string-concat.md),
  `roadmap/clusters/security/taint/sql-injection.md`
