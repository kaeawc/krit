package test

interface Cursor {
    fun moveToNext(): Boolean
    fun close()
}

interface SQLiteDatabase {
    fun rawQuery(sql: String, args: Array<String>?): Cursor
    fun query(table: String, columns: Array<String>?): Cursor
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
