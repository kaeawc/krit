package fixtures.positive.resourcecost

class DatabaseQueryOnMainThread {
    fun loadUsers(db: android.database.sqlite.SQLiteDatabase) {
        val cursor = db.rawQuery("SELECT * FROM users", null)
        cursor.close()
    }
}
