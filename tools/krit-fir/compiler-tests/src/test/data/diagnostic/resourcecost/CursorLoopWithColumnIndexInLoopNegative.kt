// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package cursorloop.negative

import android.database.Cursor
import android.annotation.SuppressLint

fun cursorLoopHoisted(cursor: Cursor) {
    val index = cursor.getColumnIndex("name")
    while (cursor.moveToNext()) cursor.getString(index)
}

fun cursorLoopOtherLoopShapes(cursor: Cursor) {
    do { cursor.getColumnIndex("name") } while (cursor.moveToNext())
    for (unused in 1..3) cursor.getColumnIndexOrThrow("name")
    while (cursor.moveToFirst()) cursor.getColumnIndex("name")
}

fun cursorLoopLocalColumn(cursor: Cursor, columns: List<String>) {
    while (cursor.moveToNext()) {
        val column = cursor.getString(0)
        cursor.getColumnIndex(column)
        columns.forEach { cursor.getColumnIndexOrThrow(it) }
        columns.forEach { name -> cursor.getColumnIndex(name) }
        for (name in columns) cursor.getColumnIndexOrThrow(name)
    }
}

fun cursorLoopConditionOnly(cursor: Cursor) {
    while (cursor.moveToNext() && cursor.getColumnIndex("name") >= 0) {
        cursor.getString(0)
    }
}

fun cursorLoopSameNameExemption(cursor: Cursor, column: String) {
    while (cursor.moveToNext()) {
        cursor.getColumnIndex(column)
        run { val column = "local"; column.length }
    }
}

fun cursorLoopSuppressed(cursor: Cursor) {
    @Suppress("CursorLoopWithColumnIndexInLoop")
    fun local() {
        while (cursor.moveToNext()) cursor.getColumnIndex("name")
    }
    local()
}

@Suppress("detekt:CursorLoopWithColumnIndexInLoop")
fun cursorLoopDetektSuppressed(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.getColumnIndex("name")
}

@Suppress("resource-cost.CursorLoopWithColumnIndexInLoop")
fun cursorLoopCategorySuppressed(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.getColumnIndex("name")
}

@Suppress("ALL")
fun cursorLoopAllSuppressed(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.getColumnIndex("name")
}

@SuppressLint(" CursorLoopWithColumnIndexInLoop ")
fun cursorLoopLintSuppressed(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.getColumnIndex("name")
}
