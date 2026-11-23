package fixtures.positive.resourcecost

import android.database.Cursor

class CursorLoopWithColumnIndexInLoop {
    fun readNames(cursor: Cursor) {
        while (cursor.moveToNext()) {
            val name = cursor.getString(cursor.getColumnIndex("name"))
            println(name)
        }
    }
}
