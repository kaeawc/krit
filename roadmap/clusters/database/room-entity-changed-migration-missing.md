# RoomEntityChangedMigrationMissing

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

`@Entity` column added/removed without a corresponding
`Migration(N-1, N)` that updates that column.

## Triggers

```kotlin
@Entity(tableName = "users")
data class User(
    @PrimaryKey val id: Long,
    val name: String,
    val avatarUrl: String, // NEW
)
// Migration(1, 2) does not reference avatarUrl
```

## Does not trigger

```kotlin
object Migration1to2 : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE users ADD COLUMN avatarUrl TEXT")
    }
}
```

## Dispatch

Cross-file: diff entity columns against all `Migration(N-1, N)`
SQL strings in the project. Requires the module index to know the
current `@Database(version = N)`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
