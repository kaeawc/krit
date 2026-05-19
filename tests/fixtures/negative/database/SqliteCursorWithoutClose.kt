package test

import android.database.sqlite.SQLiteDatabase

interface Cursor {
    fun moveToNext(): Boolean
    fun getString(idx: Int): String
    fun close()
}

inline fun <T : Cursor, R> T.use(block: (T) -> R): R = block(this)

fun loadUsersWithUse(db: SQLiteDatabase) {
    db.rawQuery("SELECT * FROM users", null).use { cursor ->
        while (cursor.moveToNext()) {
            // ...
        }
    }
}

fun loadUsersExplicitClose(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT * FROM users", null)
    while (cursor.moveToNext()) {
        // ...
    }
    cursor.close()
}

// Regression: a short-named cursor `c` is properly closed; the
// receiver-bound identifier-boundary check must recognise `c.close()`
// (and must not be confused into thinking some longer-named sibling
// closed it).
fun loadUsersShortName(db: SQLiteDatabase) {
    val c = db.rawQuery("SELECT * FROM users", null)
    try {
        while (c.moveToNext()) {
            // ...
        }
    } finally {
        c.close()
    }
}

fun interface Runnable { fun run() }

fun <T> lazyOf(init: () -> T): T = init()

// Regression: the property's static type is Runnable, not Cursor. The
// rawQuery call lives inside a nested lambda scope and does not become
// the property's value, so the rule must not fire.
fun loadUsersRunnable(db: SQLiteDatabase) {
    val handler = Runnable { db.rawQuery("SELECT * FROM users", null).close() }
    handler.run()
}

// Regression: the property's static type is `() -> Cursor`, not Cursor.
// The cursor only materialises when the caller invokes the factory.
fun loadUsersFactory(db: SQLiteDatabase) {
    val factory: () -> Cursor = { db.rawQuery("SELECT * FROM users", null) }
    factory().close()
}

// Regression: the rawQuery call sits inside a lazyOf {} lambda. The
// scope-bounded walk must not descend into the lambda body.
fun loadUsersLazy(db: SQLiteDatabase) {
    val lazyCursor = lazyOf { db.rawQuery("SELECT * FROM users", null) }
    lazyCursor.close()
}

// Regression: rawQuery lives inside a map { ... } lambda whose result is
// a List<String>, not a Cursor. The rule must not flag the property.
fun loadUserTags(db: SQLiteDatabase): List<String> {
    val tags = listOf("a", "b").map { db.rawQuery(it, null).use { c -> c.getString(0) } }
    return tags
}

// Regression for receiver-type proof: an unrelated local class with a
// method named `query` or `rawQuery` must NOT trigger the rule. Receiver
// proof requires the FQN to resolve to android.database.sqlite.SQLiteDatabase
// or androidx.sqlite.db.SupportSQLiteDatabase.
class MyCache {
    fun query(sql: String): String = ""
}

class MyDb {
    fun rawQuery(sql: String): String = ""
}

fun lookup(cache: MyCache, db: MyDb) {
    val r1 = cache.query("SELECT...")
    val r2 = db.rawQuery("SELECT...")
    println(r1)
    println(r2)
}
