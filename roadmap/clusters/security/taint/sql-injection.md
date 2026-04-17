# SqlInjection

**Cluster:** [security/taint](README.md) · **Status:** deferred ·
**Severity:** warning (when enabled)

## Catches (when substrate exists)

Untrusted input (Intent extra, Retrofit parameter, EditText text,
cursor column, HTTP header) reaching a SQL sink (`rawQuery`,
`execSQL`, `query(..., selection)`, `SimpleSQLiteQuery`).

## Shape

```kotlin
class UserActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        val id = intent.getStringExtra("id")              // TAINT SOURCE
        db.rawQuery("SELECT * FROM users WHERE id = $id") // SINK
    }
}
```

## Why deferred

Syntactic shape check ships now as
[`../call-shape/sql-injection-raw-query.md`](../call-shape/sql-injection-raw-query.md);
the taint version distinguishes a user-controlled `id` from a schema
constant `id`, which requires source→sink flow analysis.

## Substrate required

See [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
§3: source labelling, intra-procedural flow, sink registry entry for
`SQLiteDatabase.rawQuery` argument index 0.

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Tier-2 analog: [`../call-shape/sql-injection-raw-query.md`](../call-shape/sql-injection-raw-query.md)
