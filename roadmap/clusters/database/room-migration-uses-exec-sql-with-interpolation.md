# RoomMigrationUsesExecSqlWithInterpolation

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`db.execSQL("ALTER TABLE ${tableName} ADD COLUMN foo")` inside a
Migration — interpolated DDL.

## Triggers

```kotlin
object Migration1to2 : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE ${tableName} ADD COLUMN foo TEXT")
    }
}
```

## Does not trigger

```kotlin
db.execSQL("ALTER TABLE users ADD COLUMN foo TEXT")
```

## Dispatch

`call_expression` on `execSQL` inside a class extending `Migration`;
shape helper detects interpolation.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
- Related: `roadmap/clusters/security/call-shape/sql-injection-raw-query.md`
