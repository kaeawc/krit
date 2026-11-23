// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 64
package cursorloop.conservative

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase

// Both engines conservatively miss call, navigation, parenthesized, cast, and this receivers.
fun cursorLoopReceiverShapes(db: SQLiteDatabase, orders: Cursor) {
    while (orders.moveToNext()) db.rawQuery("items", null).getColumnIndex("name")
    while (orders.moveToNext()) with(db.rawQuery("items", null)) { getColumnIndex("name") }
    while (orders.moveToNext()) db.rawQuery("items", null).apply { getColumnIndex("name") }
    while (orders.moveToNext()) db.rawQuery("items", null).run { getColumnIndex("name") }
    while (orders.moveToNext()) (orders).getColumnIndex("name")
    while (orders.moveToNext()) (orders as Cursor).getColumnIndex("name")
    while (orders.moveToNext()) orders.cursorLoopIdentity().getColumnIndex("name")
    while (orders.moveToNext()) db.rawQuery("items", null).apply items@{
        orders.apply { this@items.getColumnIndex("name") }
    }
}

private fun Cursor.cursorLoopIdentity(): Cursor = this

class CursorLoopNavigation(val row: Cursor)

fun cursorLoopNavigation(cursor: Cursor, holder: CursorLoopNavigation) {
    while (cursor.moveToNext()) holder.row.getColumnIndex("name")
}

fun Cursor.cursorLoopExtensionThis() {
    while (moveToNext()) this.getColumnIndex("name")
    while (moveToNext()) also { getColumnIndex("name") }
    while (moveToNext()) let { getColumnIndex("name") }
}

fun cursorLoopStandaloneLambda(cursor: Cursor, others: List<Cursor>) {
    cursor.apply {
        while (moveToNext()) {
            val lookup: Cursor.() -> Int = { getColumnIndex("name") }
            others.forEach { println(it.lookup()) }
        }
    }
}

fun cursorLoopSetterBinding(cursor: Cursor) {
    while (cursor.moveToNext()) {
        object { var c: Cursor? = null; set(cursor) { field = cursor; cursor?.getColumnIndex("x") } }
    }
}

// Both engines deliberately miss outer for variables and non-property constructor parameters.
// Go likewise misses Java enhanced-for and try-resource bindings (covered by CursorLoop Go tests).
fun cursorLoopOuterFor(cursors: List<Cursor>) {
    for (cursor in cursors) while (cursor.moveToNext()) cursor.getColumnIndex("name")
}

class CursorLoopConstructor(cursor: Cursor) {
    init { while (cursor.moveToNext()) cursor.getColumnIndex("name") }
}

// A stable preceding var is supported, but any write or shadow makes the proof fail.
fun cursorLoopStableVar(cursor: Cursor) {
    var row = cursor
    while (cursor.moveToNext()) row.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
}

fun cursorLoopReassigned(cursor: Cursor, replacement: Cursor) {
    var row = cursor
    row = replacement
    while (cursor.moveToNext()) row.getColumnIndex("name")
}

class CursorLoopReassignedProperty(var row: Cursor) {
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) {
            this.row = cursor
            row.getColumnIndex("name")
        }
    }
}

class CursorLoopShadowedProperty(val row: Cursor) {
    fun read(cursors: List<Cursor>) {
        for (row in cursors) while (row.moveToNext()) row.getColumnIndex("name")
    }
}
