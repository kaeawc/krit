package fixtures.positive.resourcecost

class Cursor {
    fun moveToNext(): Boolean = false
    fun getString(index: Int): String = ""
    fun getColumnIndex(name: String): Int = 0
}

class CursorLoopWithColumnIndexInLoop {
    fun readNames(cursor: Cursor) {
        while (cursor.moveToNext()) {
            val name = cursor.getString(cursor.getColumnIndex("name"))
            println(name)
        }
    }
}
