// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 16, 22, 33, 37, 78, 82, 87, 119
package cursorloop.arguments

import android.database.Cursor

const val COLUMN = "name"

fun cursorArgumentVal(cursor: Cursor, first: String) {
    val column = first
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(column)
}

fun cursorArgumentCatchShadow(cursor: Cursor) {
    try {
        while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    } catch (cursor: Exception) {}
}

fun cursorArgumentCatchParameter(cursor: Cursor, column: String) {
    try {
        while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(column)
    } catch (column: Exception) {}
}

// FIR-only, resolution-proven const property; Go intentionally does not accept constants by name.
fun cursorArgumentConstant(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(COLUMN)
}

fun cursorArgumentDestructured(cursor: Cursor, pair: Pair<String, String>) {
    val (column, unused) = pair
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(column)
}

fun cursorArgumentLiteral(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("escaped\$name")
}

// Conservative misses: even unchanged vars, pure calls, indexing and interpolated strings are not whitelisted.
fun cursorArgumentMutable(cursor: Cursor, first: String) {
    var column = first
    while (cursor.moveToNext()) cursor.getColumnIndex(column)
}

fun cursorArgumentWritten(cursor: Cursor, first: String) {
    var column = first
    while (cursor.moveToNext()) {
        cursor.getColumnIndex(column)
        column = cursor.getString(0)
    }
}

fun cursorArgumentComputed(cursor: Cursor, columns: List<String>, column: String) {
    while (cursor.moveToNext()) cursor.getColumnIndex(cursor.getString(0))
    while (cursor.moveToNext()) cursor.getColumnIndex(column.uppercase())
    while (cursor.moveToNext()) cursor.getColumnIndex(columns[0])
    while (cursor.moveToNext()) cursor.getColumnIndex("$column")
    while (cursor.moveToNext()) cursor.getColumnIndex("${cursor.getString(0)}")
    while (cursor.moveToNext()) cursor.getColumnIndex(column + "suffix")
}

fun cursorArgumentWrittenIndex(cursor: Cursor, columns: List<String>) {
    var index = 0
    while (cursor.moveToNext()) {
        cursor.getColumnIndex(columns[index])
        index++
    }
}

fun cursorArgumentShadowConstant(cursor: Cursor, first: String) {
    var COLUMN = first
    while (cursor.moveToNext()) cursor.getColumnIndex(COLUMN)
}

// All three previous lexical divergences now agree, including anonymous functions.
val cursorArgumentAnonymous = fun(rows: Cursor) {
    while (rows.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
}

fun rows(rows: Cursor) {
    while (rows.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
}

fun cursorArgumentLaterShadow(cursor: Cursor, rows: Cursor, ok: Boolean) {
    if (ok) {
        while (cursor.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
    val rows = cursor
}

// The directly enclosing callable is a secondary constructor, a conservative miss in both engines.
class CursorArgumentConstructor {
    constructor(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.getColumnIndex("name")
    }
}

// FIR resolves qualified and bare constants by symbol; Go deliberately misses both.
object CursorArgumentColumns {
    const val NAME = "name"
}

const val COMPUTED_COLUMN = "na" + "me"

fun cursorArgumentConstantMisses(cursor: Cursor) {
    // FIR-only, resolution-proven object constant; Go stays silent.
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(CursorArgumentColumns.NAME)
    // FIR-only, resolution-proven compile-time concatenation; Go stays silent.
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(COMPUTED_COLUMN)
}

fun cursorArgumentDestructuredVar(cursor: Cursor, pair: Pair<String, String>) {
    var (column, unused) = pair
    while (cursor.moveToNext()) cursor.getColumnIndex(column)
}

fun cursorArgumentRawLiteral(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("""name""")
}

// Computed/delegated properties and captures across callable boundaries do not prove a stable argument.
val ordinaryColumn = "name"

fun cursorArgumentPropertyMisses(cursor: Cursor, column: String) {
    val delegated by lazy { "name" }
    while (cursor.moveToNext()) cursor.getColumnIndex(ordinaryColumn)
    while (cursor.moveToNext()) cursor.getColumnIndex(delegated)
    fun captured(rows: Cursor) {
        while (rows.moveToNext()) rows.getColumnIndex(column)
    }
    captured(cursor)
}
