// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergences from Go, each decided by the message ("Cursor '<name>' from rawQuery/query should
// be wrapped in use { } or explicitly closed with .close().").
package test

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper

// Go misses these: it proves the receiver only when it is a plain name whose type it knows, so a
// receiver that is a property chain or a call result is unknown to it. The cursor is never closed.
fun chain(helper: SQLiteOpenHelper) {
    val <!SqliteCursorWithoutClose!>cursor<!> = helper.readableDatabase.rawQuery("SELECT 1", null)
    cursor.moveToFirst()
}

fun callResult() {
    val <!SqliteCursorWithoutClose!>cursor<!> = open().rawQuery("SELECT 1", null)
    cursor.moveToFirst()
}

fun open(): SQLiteDatabase = TODO()

// Go misses this: the receiver is implicit, so it has no receiver name to prove.
fun implicitReceiver(db: SQLiteDatabase) {
    with(db) {
        val <!SqliteCursorWithoutClose!>cursor<!> = rawQuery("SELECT 1", null)
        cursor.moveToFirst()
    }
}

// Go leaves these alone: it reads the same name anywhere after the declaration as a hand-over, so a
// named argument label, a shadowing local or a lambda parameter of that name hides the leak.
// The cursor is never closed or handed over, so FIR reports it.
fun cursorArg(width: Int, cursor: Int) = width + cursor

fun namedArgumentLabel(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
    cursor.moveToFirst()
    cursorArg(width = 1, cursor = 2)
}

fun shadowedByLambdaParameter(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
    cursor.moveToFirst()
    listOf(1).forEach { cursor -> println(cursor) }
}

fun shadowedByLocal(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
    cursor.moveToFirst()
    run {
        val cursor = 3
        println(cursor)
    }
}

// Go leaves this alone: it reads any comparison as a hand-over. Nothing closes the cursor.
fun comparedWithNull(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
    if (cursor != null) {
        cursor.moveToFirst()
    }
}

// Go leaves this alone: the `!!` makes the identifier's parent a postfix expression, which it reads
// as a hand-over. The cursor is only used as a receiver and never closed.
fun assertedReceiver(db: SQLiteDatabase?) {
    val <!SqliteCursorWithoutClose!>cursor<!> = db?.rawQuery("SELECT 1", null)
    cursor!!.moveToFirst()
}

// Go leaves this alone: the query is the subject of a `when`, which has no property declaration in
// its syntax tree. Nothing closes the cursor.
fun whenSubject(db: SQLiteDatabase) {
    when (val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)) {
        else -> cursor.moveToFirst()
    }
}

// Go leaves this alone: a `let` may return the cursor, so Go reads the returned name as a
// hand-over. The property holds a column read from the cursor, which stays open.
fun letReadReturned(db: SQLiteDatabase): String? {
    val <!SqliteCursorWithoutClose!>name<!> = db.rawQuery("SELECT name FROM t", null).let { if (it.moveToFirst()) it.getString(0) else null }
    return name
}
