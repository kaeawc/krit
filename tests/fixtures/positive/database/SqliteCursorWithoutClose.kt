package test

interface Cursor {
    fun moveToNext(): Boolean
    fun close()
}

interface SQLiteDatabase {
    fun rawQuery(sql: String, args: Array<String>?): Cursor
    fun query(table: String, columns: Array<String>?): Cursor
}

fun loadUsers(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT * FROM users", null)
    while (cursor.moveToNext()) {
        // ...
    }
}

fun loadAccounts(db: SQLiteDatabase) {
    val cursor = db.query("accounts", null)
    while (cursor.moveToNext()) {
        // ...
    }
}
