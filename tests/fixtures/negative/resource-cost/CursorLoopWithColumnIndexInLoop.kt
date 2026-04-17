package fixtures.negative.resourcecost

class Cursor {
    fun moveToNext(): Boolean = false
    fun getString(index: Int): String = ""
    fun getColumnIndex(name: String): Int = 0
}

class CursorLoopWithColumnIndexInLoop {
    fun readNames(cursor: Cursor) {
        val nameIdx = cursor.getColumnIndex("name")
        while (cursor.moveToNext()) {
            val name = cursor.getString(nameIdx)
            println(name)
        }
    }
}
