package test

import android.database.sqlite.SQLiteDatabase

class SqlInjectionRawQueryFixture(private val db: SQLiteDatabase) {
    fun load(userId: String) {
        db.rawQuery("SELECT * FROM users WHERE id = $userId", null)
        db.execSQL("DELETE FROM users WHERE id = " + userId)
    }
}
