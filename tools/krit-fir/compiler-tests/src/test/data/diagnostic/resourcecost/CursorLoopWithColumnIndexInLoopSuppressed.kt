// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
@file:Suppress /* annotation trivia */ ("CursorLoopWithColumnIndexInLoop")

package cursorloop.suppressed

import android.database.Cursor

fun cursorLoopFileSuppressed(cursor: Cursor) {
    while (cursor.moveToNext()) {
        // Go misses file-annotation trivia; FIR honors the explicit suppression.
        cursor.getColumnIndex("name")
    }
}
