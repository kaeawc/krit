// Smoke: SupportSQLiteDatabase SQL execution, queries, statements, transaction {}.
package stubs

import androidx.sqlite.db.SupportSQLiteDatabase
import androidx.sqlite.db.SupportSQLiteStatement
import androidx.sqlite.db.transaction

fun migrateManually(db: SupportSQLiteDatabase) {
    db.execSQL("CREATE TABLE t (id INTEGER)")
    db.execSQL("INSERT INTO t VALUES (?)", arrayOf<Any?>(1))
    db.query("SELECT * FROM t").use { it.moveToFirst() }
    db.query("SELECT * FROM t WHERE id = ?", arrayOf<Any?>(1)).close()
    db.transaction { execSQL("DELETE FROM t") }
    val stmt: SupportSQLiteStatement = db.compileStatement("DELETE FROM t WHERE id = ?")
    stmt.bindLong(1, 1L)
    stmt.bindString(2, "x")
    <!PrintlnInProduction!>println<!>(stmt.executeUpdateDelete() + db.version)
    if (db.inTransaction()) db.endTransaction()
}
