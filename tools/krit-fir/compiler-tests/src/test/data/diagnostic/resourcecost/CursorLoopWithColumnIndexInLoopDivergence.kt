// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 22, 38, 46, 53, 65, 93, 101, 109, 119, 130, 139
package cursorloop.divergence

import android.database.Cursor as AndroidCursor

class CursorLoopLookalike {
    fun moveToNext(): Boolean = false
    fun getColumnIndex(name: String): Int = name.length
}

fun cursorLoopLookalikeOnly(cursor: CursorLoopLookalike) {
    while (cursor.moveToNext()) {
        // Go reports by name; FIR drops a resolved non-Android receiver.
        cursor.getColumnIndex("name")
    }
}

fun cursorLoopLookalikeFirst(cursor: AndroidCursor, other: CursorLoopLookalike) {
    while (cursor.moveToNext()) {
        // Go chooses this lookalike; FIR drops it and does not select a later call.
        other.getColumnIndex("name")
        cursor.getColumnIndex("real")
    }
}

fun cursorLoopDestructuredColumn(cursor: AndroidCursor, names: List<Pair<String, String>>) {
    while (cursor.moveToNext()) {
        for ((column, unused) in names) {
            // Both engines conservatively keep destructured for variables loop-local.
            cursor.getColumnIndex(column)
        }
    }
}

fun cursorLoopNullable(cursor: AndroidCursor?) {
    while (cursor?.moveToNext() == true) {
        cursor?.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
}

typealias CursorLoopAlias = AndroidCursor

fun cursorLoopAlias(cursor: CursorLoopAlias) {
    while (cursor.moveToNext()) {
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>("name")
    }
}

fun cursorLoopUnrelatedCondition(cursor: AndroidCursor, other: CursorLoopLookalike) {
    while (other.moveToNext()) {
        // Go accepts a lookalike iteration method; FIR requires a resolved Cursor iteration.
        cursor.getColumnIndex("name")
    }
}

interface CursorLoopOverload : AndroidCursor {
    fun getColumnIndex(index: Int): Int
}

fun cursorLoopCustomOverload(cursor: CursorLoopOverload) {
    while (cursor.moveToNext()) {
        // Both engines reject the numeric argument and select the next literal lookup.
        cursor.getColumnIndex(1)
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("real")
    }
}

fun cursorLoopNestedParameter(cursor: AndroidCursor, column: String) {
    while (cursor.moveToNext()) {
        // Both engines conservatively exempt any body-local parameter name.
        cursor.getColumnIndex(column)
        fun nested(column: String): Int = column.length
        nested("local")
    }
}

abstract class CursorLoopMemberExtension : AndroidCursor {
    fun String.getColumnIndex(column: String): Int = length + column.length

    fun readLookalikeExtension() {
        while (moveToNext()) {
            // Both engines conservatively miss non-simple explicit receivers, including this String literal.
            "text".getColumnIndex("name")
        }
    }
}

fun cursorLoopSiblingForBinding(cursor: AndroidCursor, column: String, columns: List<String>) {
    for (column in columns) println(column)
    while (cursor.moveToNext()) {
        // FIR abstains: a preceding for variable is scoped to its loop.
        cursor.getColumnIndex(column)
    }
}

fun cursorLoopSiblingFunctionParameter(cursor: AndroidCursor) {
    fun log(cursor: String) = println(cursor)
    while (cursor.moveToNext()) {
        // FIR abstains: a preceding local function parameter is function-local.
        cursor.getColumnIndex("name")
    }
}

fun cursorLoopSiblingFunctionName(cursor: AndroidCursor, columns: List<String>) {
    fun cursor() = columns.size
    while (cursor.moveToNext()) {
        // FIR abstains: a local function name does not bind the outer cursor value.
        cursor.getColumnIndex("name")
    }
}

fun cursorLoopSiblingFinallyCatch(cursor: AndroidCursor) {
    try {
    } catch (cursor: Exception) {
    } finally {
        while (cursor.moveToNext()) {
            // FIR abstains: an earlier catch parameter is not visible in finally.
            cursor.getColumnIndex("name")
        }
    }
}

fun cursorLoopSiblingLaterCatch(cursor: AndroidCursor) {
    try {
    } catch (cursor: java.io.IOException) {
    } catch (error: Exception) {
        while (cursor.moveToNext()) {
            // FIR abstains: an earlier catch parameter is not visible in another catch.
            cursor.getColumnIndex("name")
        }
    }
}

fun cursorLoopSiblingClassName(cursor: AndroidCursor) {
    class cursor
    while (cursor.moveToNext()) {
        // FIR abstains: a local class name is not a value binding for the outer cursor.
        cursor.getColumnIndex("name")
    }
}

fun cursorLoopWhenSubjectLambdaReceiver(cursor: AndroidCursor, names: List<String>) {
    when (names.firstOrNull { cursor -> cursor.isNotEmpty() }) {
        // FIR-only: the lambda parameter is not visible here; Go's when-subject walk treats it as a binding.
        else -> while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
}

fun cursorLoopWhenSubjectLambdaArgument(c: AndroidCursor, column: String, names: List<String>) {
    when (names.firstOrNull { column -> column.isNotEmpty() }) {
        // FIR-only: the lambda parameter is not visible here; Go's when-subject walk treats it as a binding.
        else -> while (c.moveToNext()) c.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(column)
    }
}

fun cursorLoopParenthesizedLambdaReceiver(cursor: AndroidCursor, names: List<String>) {
    (names.forEach { cursor -> println(cursor) })
    while (cursor.moveToNext()) {
        // FIR-only: the earlier lambda parameter is not visible here; Go's parenthesized-expression walk treats it as a binding.
        cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
}
