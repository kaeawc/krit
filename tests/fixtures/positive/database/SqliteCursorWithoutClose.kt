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

// Regression: a leaked short-named cursor `c` must FIRE even when a
// longer-named sibling `vc.close()` is present in the same scope. The
// substring scan that this rule used to use matched `vc.close(` as
// evidence that `c` had been closed.
fun loadUsersWithLookalike(db: SQLiteDatabase) {
    val c = db.rawQuery("SELECT * FROM users", null)
    val vc = db.rawQuery("SELECT * FROM admins", null)
    while (c.moveToNext()) {
        // ...
    }
    vc.close()
    // c is never closed
}

// Regression for scope-bounded walk: the rawQuery call sits on the
// property initializer's outer call/navigation chain (top-level value),
// so it must still FIRE even though the chain crosses a navigation
// expression. Locks in coverage that the scope-aware walk did not
// over-prune the legitimate case.
fun loadUsersChain(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT * FROM users WHERE active = 1", null)
    while (cursor.moveToNext()) {
        // never closed
    }
}
