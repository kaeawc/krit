// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package cursorloop.constants

import android.database.Cursor
import java.io.File.separator
import java.util.jar.JarFile.MANIFEST_NAME

const val COLUMN = "name"

class ChangingColumns(private val names: Iterator<String>) {
    val COLUMN: String get() = names.next()
}

// Implicit receiver members win over the top-level constant, even with custom getters.
fun ChangingColumns.read(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.getColumnIndex(COLUMN)
}

fun receiverLambda(cursor: Cursor, columns: ChangingColumns) {
    with(columns) {
        val rows = cursor
        while (rows.moveToNext()) rows.getColumnIndex(COLUMN)
    }
}

class CompanionShadow {
    companion object {
        val COLUMN: String get() = System.nanoTime().toString()
    }
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.getColumnIndex(COLUMN)
    }
}

// Class members are visible irrespective of declaration order.
class LaterMember {
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.getColumnIndex(COLUMN)
    }
    val COLUMN: String get() = System.nanoTime().toString()
}

class ConstructorShadow(var COLUMN: String) {
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.getColumnIndex(COLUMN)
    }
}

open class OpenConstructorShadow(open val COLUMN: String) {
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.getColumnIndex(COLUMN)
    }
}

open class ColumnBase {
    val COLUMN: String get() = System.nanoTime().toString()
}

class InheritedShadow : ColumnBase() {
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.getColumnIndex(COLUMN)
    }
}

// FIR-only, resolution-proven top-level const declared after the function; Go stays silent.
fun laterConstant(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(LATER_COLUMN)
}
const val LATER_COLUMN = "name"

class CompanionConstant {
    companion object {
        const val COLUMN = "name"
    }
    // FIR-only, resolution-proven companion const; Go stays silent.
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(COLUMN)
    }
}

object ObjectConstant {
    const val COLUMN = "name"
    // FIR-only, resolution-proven object const; Go stays silent.
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(COLUMN)
    }
}

// FIR-only, resolution-proven Java static final String with a class-file constant initializer.
fun javaConstant(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(MANIFEST_NAME)
}

// Both engines stay silent on a non-constant Java static final String.
fun javaRuntimeField(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.getColumnIndex(separator)
}
