// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives the Go rule also leaves alone: a cursor closed or used in its scope (however deeply
// nested the close is), a cursor consumed on the spot, a cursor handed to other code, a call
// that is not a database query, and queries in another scope.
package test

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase

fun closed(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT 1", null)
    cursor.moveToFirst()
    cursor.close()
}

fun used(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT 1", null)
    cursor.use { it.moveToFirst() }
}

fun finallyClose(db: SQLiteDatabase) {
    val c = db.rawQuery("SELECT 1", null)
    try {
        c.moveToFirst()
    } finally {
        c.close()
    }
}

fun nestedClose(db: SQLiteDatabase, flag: Boolean) {
    val cursor = db.rawQuery("SELECT 1", null)
    if (flag) {
        listOf(1).forEach { _ -> cursor.close() }
    }
}

fun onTheSpot(db: SQLiteDatabase) {
    val names = db.rawQuery("SELECT 1", null).use { it.getString(0) }
    println(names)
}

fun inline(db: SQLiteDatabase) {
    db.rawQuery("SELECT 1", null).use { cursor ->
        cursor.moveToFirst()
    }
}

// The cursor is returned, stored, or passed on: its new owner closes it.
fun returned(db: SQLiteDatabase): Cursor {
    val cursor = db.rawQuery("SELECT 1", null)
    return cursor
}

fun expressionBody(db: SQLiteDatabase): Cursor {
    val cursor = db.rawQuery("SELECT 1", null)
    return run { cursor }
}

class Holder {
    var stored: Cursor? = null
}

fun stored(db: SQLiteDatabase, holder: Holder) {
    val cursor = db.rawQuery("SELECT 1", null)
    holder.stored = cursor
}

fun consume(cursor: Cursor) {
    cursor.close()
}

fun passedOn(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT 1", null)
    consume(cursor)
}

fun collected(db: SQLiteDatabase, all: MutableList<Cursor>) {
    val cursor = db.rawQuery("SELECT 1", null)
    all.add(cursor)
}

// The query sits in another scope: a lambda, a local function, a nested object.
fun lazyOf(init: () -> Cursor): Cursor = init()

fun inLambda(db: SQLiteDatabase) {
    val handler = Runnable { db.rawQuery("SELECT 1", null).close() }
    handler.run()
}

fun factory(db: SQLiteDatabase) {
    val make: () -> Cursor = { db.rawQuery("SELECT 1", null) }
    make().close()
}

fun deferred(db: SQLiteDatabase) {
    val lazyCursor = lazyOf { db.rawQuery("SELECT 1", null) }
    lazyCursor.close()
}

fun mapped(db: SQLiteDatabase): List<String> {
    val tags = listOf("a", "b").map { db.rawQuery(it, null).use { c -> c.getString(0) } }
    return tags
}

fun localFunction(db: SQLiteDatabase) {
    val result = fun(): Int = db.rawQuery("SELECT 1", null).use { it.count }
    println(result())
}

// A same-named method of another class is not a database query.
class MyCache {
    fun query(sql: String): String = ""
}

class MyDb {
    fun rawQuery(sql: String): String = ""
}

fun lookup(cache: MyCache, db: MyDb) {
    val r1 = cache.query("SELECT...")
    val r2 = db.rawQuery("SELECT...")
    println(r1)
    println(r2)
}

// Other query calls on other types.
fun nonDatabase(helper: Helper) {
    val rows = helper.query("table")
    println(rows)
}

class Helper {
    fun query(table: String): List<String> = emptyList()
}

// The cursor was never opened by a property.
fun noProperty(db: SQLiteDatabase) {
    db.rawQuery("SELECT 1", null).close()
}

fun parameterDefault(db: SQLiteDatabase, cursor: Cursor? = null) {
    cursor?.close()
    db.isOpen
}
