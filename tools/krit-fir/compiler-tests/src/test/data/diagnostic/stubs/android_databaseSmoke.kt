// Smoke: iterate a Cursor inside use {} as real code does.
package stubs

import android.database.Cursor

fun readNames(cursor: Cursor): List<String> {
    val names = mutableListOf<String>()
    cursor.use { c ->
        val column = c.getColumnIndexOrThrow("name")
        while (c.moveToNext()) {
            if (!c.isNull(column)) names += c.getString(column) ?: ""
        }
        println(c.count + c.getInt(c.getColumnIndex("age")) + c.getLong(0))
    }
    return names
}
