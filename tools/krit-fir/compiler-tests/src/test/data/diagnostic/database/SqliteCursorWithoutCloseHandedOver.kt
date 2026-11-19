// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 18, 23, 38
// A property holding a value read from the cursor (its row count, a column, a
// Boolean) leaves the cursor open whatever happens to that value, so returning
// or passing that value on hands nothing over. A cursor returned through a
// call that returns its receiver is handed over.
package test

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase

fun countReturned(db: SQLiteDatabase): Int {
    val <!SqliteCursorWithoutClose!>n<!> = db.rawQuery("SELECT * FROM t", null).count
    return n
}

fun booleanPassedOn(db: SQLiteDatabase) {
    val <!SqliteCursorWithoutClose!>any<!> = db.rawQuery("SELECT 1", null).moveToFirst()
    println(any)
}

fun safeCallReadReturned(db: SQLiteDatabase?): Int? {
    val <!SqliteCursorWithoutClose!>columns<!> = db?.rawQuery("SELECT 1", null)?.count
    return columns
}

fun returnedThroughApply(db: SQLiteDatabase): Cursor {
    val cursor = db.rawQuery("SELECT 1", null)
    return cursor.apply { moveToFirst() }
}

fun returnedThroughAlso(db: SQLiteDatabase): Cursor? {
    val cursor = db.rawQuery("SELECT 1", null)
    return cursor.also { it.moveToFirst() }.takeIf { it.count > 0 }
}

fun readThroughApply(db: SQLiteDatabase): Int {
    val <!SqliteCursorWithoutClose!>cursor<!> = db.rawQuery("SELECT 1", null)
    return cursor.apply { moveToFirst() }.count
}
