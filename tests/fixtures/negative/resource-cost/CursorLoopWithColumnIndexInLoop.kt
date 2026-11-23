package fixtures.negative.resourcecost

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase

class CursorLoopWithColumnIndexInLoop {
    fun readNames(cursor: Cursor) {
        val nameIdx = cursor.getColumnIndex("name")
        while (cursor.moveToNext()) {
            val name = cursor.getString(nameIdx)
            println(name)
        }
    }

    fun readNested(db: SQLiteDatabase, orders: Cursor) {
        while (orders.moveToNext()) {
            val items = db.rawQuery("SELECT name FROM items", null)
            val index = items.getColumnIndex("name")
            while (items.moveToNext()) println(items.getString(index))
            items.close()
            db.rawQuery("SELECT name FROM items", null).use { row -> row.getColumnIndex("name") }
            with(items) { getColumnIndex("name") }
            items.apply { getColumnIndex("name"); close() }
            items.run { getColumnIndex("name") }
        }
    }
}
