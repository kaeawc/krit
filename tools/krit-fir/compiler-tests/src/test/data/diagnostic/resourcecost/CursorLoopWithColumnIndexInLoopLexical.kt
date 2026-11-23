// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 74, 90, 94, 97
package cursorloop.lexical

import android.database.Cursor

// Properties, including constructor properties, computed getters and delegates, are conservative misses.
class CursorLoopLexicalProperties(private val items: Cursor, private val pages: List<Cursor>) {
    private var page = 0
    private val stored: Cursor = items
    private val current: Cursor get() = pages[page]
    private val delegated: Cursor by lazy { items }

    fun read(cursor: Cursor, rows: List<CursorLoopLexicalRow>) {
        while (cursor.moveToNext()) items.getColumnIndex("name")
        while (cursor.moveToNext()) stored.getColumnIndex("name")
        while (cursor.moveToNext()) delegated.getColumnIndex("name")
        while (cursor.moveToNext()) {
            page = cursor.getInt(0)
            current.getColumnIndex("name")
        }
        while (cursor.moveToNext()) {
            with(rows[cursor.getInt(0)]) { items.getColumnIndex("name") }
        }
    }
}

class CursorLoopLexicalRow(val items: Cursor)

private val shared: Cursor = TODO()

// Top-level values are missed even when immutable and apparently stable.
fun cursorLoopTopLevel(cursor: Cursor) {
    while (cursor.moveToNext()) shared.getColumnIndex("name")
}

// Any nested execution/receiver scope between the loop and lookup invalidates the lexical proof.
fun cursorLoopNestedExecution(cursor: Cursor) {
    while (cursor.moveToNext()) run { cursor.getColumnIndex("name") }
    while (cursor.moveToNext()) with(cursor) { cursor.getColumnIndex("name") }
    while (cursor.moveToNext()) {
        val lookup = { cursor.getColumnIndex("name") }
        lookup()
    }
    while (cursor.moveToNext()) {
        fun lookup() = cursor.getColumnIndex("name")
        lookup()
    }
    while (cursor.moveToNext()) {
        object {
            init { cursor.getColumnIndex("name") }
            fun lookup() = cursor.getColumnIndex("name")
        }.lookup()
    }
}

// Captured values from another function/lambda are not accepted as bindings for the inner loop.
fun cursorLoopCaptured(cursor: Cursor) {
    val outer = cursor
    run { while (cursor.moveToNext()) outer.getColumnIndex("name") }
    fun nested() { while (cursor.moveToNext()) outer.getColumnIndex("name") }
    nested()
}

class CursorLoopLexicalException(cursor: Cursor) : Exception(), Cursor by cursor

// Enclosing for/destructuring/catch variables stay conservative; direct lambda parameters are stable.
fun cursorLoopEnclosingShadows(cursor: Cursor, rows: Cursor, cursors: List<Cursor>, pairs: List<Pair<Int, Cursor>>) {
    for (rows in cursors) while (cursor.moveToNext()) rows.getColumnIndex("name")
    for ((index, rows) in pairs) while (cursor.moveToNext()) rows.getColumnIndex("name")
    try { println(cursor) } catch (rows: CursorLoopLexicalException) {
        while (cursor.moveToNext()) rows.getColumnIndex("name")
    }
    cursors.forEach { rows -> while (cursor.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
    when (val rows = cursor) {
        else -> while (cursor.moveToNext()) rows.getColumnIndex("name")
    }
}

// A delegated local is not a stable value binding either.
fun cursorLoopLocalDelegate(cursor: Cursor) {
    val row: Cursor by lazy { cursor }
    while (cursor.moveToNext()) row.getColumnIndex("name")
}

// Ordinary earlier locals remain supported in the function/lambda that directly owns the loop.
fun cursorLoopNearestLocals(cursor: Cursor) {
    val row = cursor
    if (cursor.moveToNext()) {
        while (cursor.moveToNext()) row.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
    run {
        val local = cursor
        while (local.moveToNext()) local.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
    fun nested(local: Cursor) {
        while (local.moveToNext()) local.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
    nested(cursor)
}
