// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 17, 24, 31, 55, 62, 67, 72, 78, 86, 103
package cursorloop.positive

import android.database.Cursor

fun cursorLoopBasic(cursor: Cursor) {
    while (cursor.moveToNext()) {
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
        cursor.getColumnIndexOrThrow("age")
    }
}

fun cursorLoopUse(cursor: Cursor) {
    cursor.use { row ->
        while (row.moveToNext()) {
            row.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>("name")
        }
    }
}

fun cursorLoopOuterColumn(cursor: Cursor, column: String, ready: Boolean) {
    while (ready && cursor.moveToNext()) {
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(column)
    }
}

fun cursorLoopSkipDynamicFirst(cursor: Cursor, columns: List<String>) {
    while (cursor.moveToNext()) {
        for (column in columns) cursor.getColumnIndex(column)
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>("name")
    }
}

fun cursorLoopNestedScope(cursor: Cursor) {
    while (cursor.moveToNext()) {
        run { cursor.getColumnIndex("name") }
    }
}

fun cursorLoopImplicit(cursor: Cursor) {
    with(cursor) {
        while (moveToNext()) {
            getColumnIndex("name")
        }
    }
}

interface CursorLoopSubtype : Cursor {
    override fun getColumnIndex(name: String): Int
}

fun cursorLoopInherited(cursor: CursorLoopSubtype) {
    while (cursor.moveToNext()) {
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
}

fun cursorLoopMultiline(cursor: Cursor) {
    while (cursor.moveToNext()) {
        cursor
            .<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
}

fun cursorLoopSingleStatement(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
}

fun cursorLoopDifferentReceiver(cursor: Cursor, other: Cursor) {
    while (cursor.moveToNext()) {
        other.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
}

fun cursorLoopLabelled(cursor: Cursor) {
    rows@ while (cursor.moveToNext()) {
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>("name")
        break@rows
    }
}

fun cursorLoopSmartCast(candidate: Any) {
    if (candidate is Cursor) {
        while (candidate.moveToNext()) {
            candidate.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
        }
    }
}

fun cursorLoopChainedReceiver(cursor: Cursor) {
    while (cursor.moveToNext()) {
        // Both engines conservatively miss all chained receiver expressions.
        cursor.also { it.getColumnIndex("inner") }
            .getColumnIndex("outer")
    }
}

fun cursorLoopNestedArgument(cursor: Cursor) {
    while (cursor.moveToNext()) {
        // The outer call has a computed argument; only the nested literal lookup can be hoisted.
        cursor.getColumnIndex(
            cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("inner").toString()
        )
    }
}

fun cursorLoopAnonymousSubtype(cursor: Cursor) {
    val local = object : Cursor by cursor {}
    while (local.moveToNext()) {
        // Go misses this anonymous delegated receiver; FIR proves its Cursor lookup and iteration symbols.
        local.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>("name")
    }
}

// Both engines intentionally miss outer use parameters, implicit with receivers, and lookups inside lambdas.
