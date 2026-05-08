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
