// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 53, 60, 66, 73, 82
package cursorloop.receivers

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase

fun cursorLoopNestedQuery(db: SQLiteDatabase, orders: Cursor) {
    while (orders.moveToNext()) {
        val items: Cursor = db.rawQuery("SELECT name FROM items WHERE order_id = ?", arrayOf(orders.getString(0)))
        val nameIdx = items.getColumnIndex("name")
        while (items.moveToNext()) println(items.getString(nameIdx))
        items.close()
    }
}

fun cursorLoopLocalScopeReceivers(db: SQLiteDatabase, orders: Cursor) {
    while (orders.moveToNext()) {
        db.query("items", null, null, null, null, null, null).use { c -> c.getColumnIndex("name") }
        db.rawQuery("SELECT name FROM items", null).use { c -> c.getColumnIndex("name") }
        db.rawQuery("SELECT name FROM items", null).use { it.getColumnIndex("name") }
        val localCursor = db.rawQuery("SELECT name FROM items", null)
        with(localCursor) { getColumnIndex("x") }
        localCursor.apply { getColumnIndex("x") }
        localCursor.run { getColumnIndex("x") }
        localCursor.apply { this.getColumnIndex("x") }
        localCursor.apply { run { getColumnIndex("x") } }
    }
}

fun cursorLoopExternalScopeReceivers(orders: Cursor) {
    while (orders.moveToNext()) {
        with(orders) { getColumnIndex("x") }
    }
    while (orders.moveToNext()) {
        orders.apply { getColumnIndex("x") }
    }
    while (orders.moveToNext()) {
        orders.run { getColumnIndex("x") }
    }
    while (orders.moveToNext()) {
        orders.apply { this.getColumnIndex("x") }
    }
    while (orders.moveToNext()) {
        orders.apply { run { getColumnIndex("x") } }
    }
}

fun cursorLoopSkipsLocalReceiver(orders: Cursor) {
    while (orders.moveToNext()) {
        val local = orders
        local.getColumnIndex("local")
        orders.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("outer")
    }
}

fun cursorLoopSkipsBoundReceiver(orders: Cursor) {
    while (orders.moveToNext()) {
        with(orders) { getColumnIndex("bound") }
        orders.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("outer")
    }
}

fun cursorLoopReceiverBeforeArguments(cursor: Cursor) {
    while (cursor.moveToNext()) {
        cursor.getString(cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("first"))
            .plus(cursor.getString(cursor.getColumnIndex("last")))
    }
}

fun cursorLoopReceiverBeforeLambda(cursor: Cursor) {
    while (cursor.moveToNext()) {
        cursor.getString(cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("first"))
            .also { println(cursor.getColumnIndex("last")) }
    }
}

private fun Cursor.cursorLoopIdentity(index: Int): Cursor = this

fun cursorLoopCallBeforeReceiver(cursor: Cursor) {
    while (cursor.moveToNext()) {
        cursor.cursorLoopIdentity(cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("inner"))
            .getColumnIndex("outer")
    }
}

fun cursorLoopExplicitEmptyLambda(cursor: Cursor, columns: List<String>) {
    columns.forEach {
        while (cursor.moveToNext()) {
            // Both engines miss lookups inside lambdas, even with explicit empty parameter lists.
            run { -> cursor.getColumnIndex(it) }
        }
    }
}

// Both engines intentionally miss implicit/this receivers, even when an outer scope is invariant.
// Both engines miss chained receivers; a nested simple-name lookup may still be selected.
