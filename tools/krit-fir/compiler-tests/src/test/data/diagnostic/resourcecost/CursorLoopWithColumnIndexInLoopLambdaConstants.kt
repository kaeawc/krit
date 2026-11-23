// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 31, 37, 42, 46, 50, 54, 58, 62, 66, 70
package cursorloop.lambdaconstants

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase
import android.provider.Settings
import android.provider.Settings as PlatformSettings
import android.provider.Settings.Global.AIRPLANE_MODE_ON
import java.util.jar.JarFile

// Both engines accept a qualified platform constant; FIR proves the Java field's constant initializer.
fun nullableQuery(db: SQLiteDatabase) {
    db.query("items", null, null, null, null, null, null)?.use { cursor ->
        while (cursor.moveToNext()) {
            val value = cursor.getString(cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>(PlatformSettings.Global.AIRPLANE_MODE_ON))
        }
    }
}

// FIR-only: the imported Java field has the same resolved constant initializer.
fun importedColumn(db: SQLiteDatabase) {
    db.rawQuery("SELECT name FROM items", null)?.use { cursor ->
        while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(AIRPLANE_MODE_ON)
    }
}

// Both engines accept a qualified platform constant backed by a class-file constant initializer.
fun libraryColumn(db: SQLiteDatabase) {
    db.rawQuery("SELECT name FROM items", null)?.use { cursor ->
        while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(JarFile.MANIFEST_NAME)
    }
}

fun literalUse(db: SQLiteDatabase) {
    db.rawQuery("SELECT name FROM items", null)?.use { cursor ->
        while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name")
    }
}

fun literalLet(cursor: Cursor) {
    cursor.let { rows -> while (rows.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
}

fun literalAlso(cursor: Cursor) {
    cursor.also { rows -> while (rows.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
}

fun literalForEach(cursors: List<Cursor>) {
    cursors.forEach { rows -> while (rows.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
}

fun labeledTrailingLambda(cursors: List<Cursor>) {
    cursors.forEach outer@{ cursor -> while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
}

fun annotatedTrailingLambda(cursors: List<Cursor>) {
    cursors.forEach @Suppress("UNUSED_VARIABLE") { cursor -> while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
}

fun labeledWhileLoop(cursor: Cursor) {
    cursor.use { row -> rows@ while (row.moveToNext()) row.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
}

fun annotatedWhileLoop(cursor: Cursor) {
    cursor.use { row -> @Suppress("UNUSED_VARIABLE") while (row.moveToNext()) row.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") }
}

fun soleArgument(cursor: Cursor) {
    cursor.use({ rows -> while (rows.moveToNext()) rows.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>("name") })
}

fun labeledSoleArgument(cursor: Cursor) {
    cursor.use(rows@{ row -> while (row.moveToNext()) row.getColumnIndex("name") })
}

fun annotatedSoleArgument(cursor: Cursor) {
    cursor.use(@Suppress("UNUSED_VARIABLE") { row -> while (row.moveToNext()) row.getColumnIndex("name") })
}

// Both engines conservatively miss implicit it: syntax alone cannot prove a value parameter.
fun implicitParameter(cursor: Cursor) {
    cursor.use { while (it.moveToNext()) it.getColumnIndex("name") }
}

class ImplicitReceiverMember(var it: Cursor) {
    fun read() { run { while (it.moveToNext()) it.getColumnIndex("name") } }
}

fun lambdaBoundaries(db: SQLiteDatabase, ready: Boolean) {
    db.rawQuery("SELECT name FROM items", null).use { cursor ->
        if (ready) { while (cursor.moveToNext()) cursor.getColumnIndex("name") }
        run { while (cursor.moveToNext()) cursor.getColumnIndex("name") }
        fun nested() { while (cursor.moveToNext()) cursor.getColumnIndex("name") }
        nested()
        val reader = object { fun read() { while (cursor.moveToNext()) cursor.getColumnIndex("name") } }
        while (cursor.moveToNext()) run { cursor.getColumnIndex("name") }
        while (cursor.moveToNext()) {
            val cursor = db.rawQuery("SELECT name FROM items", null)
            cursor.getColumnIndex("name")
        }
    }
    val callback = { cursor: Cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }
}

class DynamicColumns {
    val AIRPLANE_MODE_ON: String get() = System.nanoTime().toString()
}

// An uppercase last segment and an imported class name cannot prove a constant.
fun shadowedQualified(db: SQLiteDatabase, columns: DynamicColumns) {
    val Settings = columns
    db.rawQuery("SELECT name FROM items", null).use { cursor ->
        while (cursor.moveToNext()) cursor.getColumnIndex(Settings.AIRPLANE_MODE_ON)
    }
}

object ConstantColumns {
    const val LOOKUP_KEY = "name"
}

fun computedQualifier(cursor: Cursor, columns: () -> ConstantColumns) {
    while (cursor.moveToNext()) cursor.getColumnIndex(columns().LOOKUP_KEY)
}
