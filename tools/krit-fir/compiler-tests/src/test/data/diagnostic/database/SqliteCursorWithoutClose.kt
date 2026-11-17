// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 19, 24, 29, 35, 46, 53, 58, 64, 68, 75, 81, 83, 88, 95, 101, 107
// Positive: a property that opens a cursor with rawQuery/query and never closes it or hands it
// over. Each is reported on the property's name, the line the Go rule reports.
package test

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase
import androidx.sqlite.db.SupportSQLiteDatabase

fun rawQuery(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT * FROM users", null)
    while (cursor.moveToNext()) {
        cursor.getString(0)
    }
}

fun query(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.query("accounts", null, null, null, null, null, null)
    cursor.moveToFirst()
}

fun supportDatabase(db: SupportSQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.query("SELECT * FROM users")
    cursor.moveToFirst()
}

fun variable(db: SQLiteDatabase) {
    var <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
    cursor.moveToFirst()
}

// A short-named cursor is not closed by a longer-named sibling's close.
fun sibling(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>c<!> = db.rawQuery("SELECT * FROM users", null)
    val vc = db.rawQuery("SELECT * FROM admins", null)
    c.moveToNext()
    vc.close()
}

// A close before the declaration is not a later cleanup.
fun closedBefore(db: SQLiteDatabase, other: Cursor) {
    other.close()
    run {
        other.close()
        val <!SqliteCursorWithoutClose!>other<!> = db.rawQuery("SELECT 1", null)
        other.moveToFirst()
    }
}

// The rows are read but the cursor behind them stays open.
fun count(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>count<!> = db.rawQuery("SELECT 1", null).count
    count.toString()
}

fun safeCall(db: SQLiteDatabase?) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db?.rawQuery("SELECT 1", null)
    cursor?.moveToFirst()
}

fun annotated(db: SQLiteDatabase) {
    @Suppress("UNUSED_VARIABLE")
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
}

fun backticked(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>`the cursor`<!> = db.rawQuery("SELECT 1", null)
    `the cursor`.moveToFirst()
}

// Cursors made in a lambda body are checked in that scope.
fun inLambda(db: SQLiteDatabase) {
    listOf("SELECT 1").forEach { sql ->
        val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery(sql, null)
        cursor.moveToFirst()
    }
}

class Repository(private val db: SQLiteDatabase) {
    private val <!SqliteCursorWithoutClose!>all<!> = db.rawQuery("SELECT * FROM t", null)

    val <!SqliteCursorWithoutClose!>fresh<!>: Cursor get() = db.rawQuery("SELECT 1", null)

    /**
     * KDoc is not part of the reported line.
     */
    internal val <!SqliteCursorWithoutClose!>documented<!> = db.rawQuery("SELECT 1", null)

    fun read() {
        all.moveToFirst()
    }
}

val <!SqliteCursorWithoutClose!>topLevel<!> = topLevelDatabase.rawQuery("SELECT 1", null)

val topLevelDatabase: SQLiteDatabase get() = TODO()

fun localClass(db: SQLiteDatabase) {
    class Local {
        val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
    }
    Local()
}

fun anonymousObject(db: SQLiteDatabase): Any = object {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
}
