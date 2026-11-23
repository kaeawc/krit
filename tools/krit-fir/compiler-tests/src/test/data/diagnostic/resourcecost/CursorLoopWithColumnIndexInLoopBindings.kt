// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 25
package cursorloop.bindings

import android.database.Cursor

fun cursorLoopBindingReceivers(cursor: Cursor, children: Map<String, Cursor>, pair: Pair<String, Cursor>) {
    while (cursor.moveToNext()) {
        for ((name, child) in children) child.getColumnIndex("name")
        children.forEach { (name, child) -> child.getColumnIndex("name") }
        val (name, child) = pair
        child.getColumnIndex("name")
        fun idx(child: Cursor) = child.getColumnIndex("name")
        idx(cursor)
    }
}

fun cursorLoopOuterBindings(cursor: Cursor) {
    val outer = cursor
    while (cursor.moveToNext()) {
        outer.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
    cursor.use { row ->
        while (row.moveToNext()) {
            row.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
        }
    }
}

class CursorLoopPropertyBinding(private val outer: Cursor) {
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) {
            outer.getColumnIndex("name")
        }
    }
}

fun Cursor.cursorLoopUseDoesNotBindReceiver() {
    while (moveToNext()) {
        use { getColumnIndex("name") }
    }
}

fun Cursor.cursorLoopExplicitThisScope() {
    while (moveToNext()) {
        with(this) { this.getColumnIndex("name") }
    }
}

private fun Cursor.cursorLoopCustomScope(block: Cursor.() -> Unit) = block()

fun Cursor.cursorLoopUnknownImplicitBinding() {
    while (moveToNext()) {
        val local = this
        local.cursorLoopCustomScope { getColumnIndex("name") }
    }
}

// Both engines intentionally miss properties, outer lambda bindings, extension this, and explicit this scopes.
