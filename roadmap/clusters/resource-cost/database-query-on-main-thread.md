# DatabaseQueryOnMainThread

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`SQLiteDatabase.query/.rawQuery(...)` inside a non-`suspend`
function that isn't wrapped in `withContext(Dispatchers.IO)`.

## Triggers

```kotlin
fun load(): Cursor = db.rawQuery("SELECT * FROM users", null)
```

## Does not trigger

```kotlin
suspend fun load(): Cursor = withContext(Dispatchers.IO) {
    db.rawQuery("SELECT * FROM users", null)
}
```

## Dispatch

`call_expression` on `rawQuery`/`query` inside a non-suspend
function with no dispatcher switch.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
