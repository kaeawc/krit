package test

import android.database.sqlite.SQLiteDatabase

private const val USERS_TABLE = "users"
private const val COLUMN_ID = "id"

class SqlInjectionRawQuerySafeFixture(private val db: SQLiteDatabase) {
    fun load(userId: String) {
        db.rawQuery("SELECT * FROM users WHERE id = ?", arrayOf(userId))
        db.rawQuery("SELECT * FROM $USERS_TABLE WHERE $COLUMN_ID = ?", arrayOf(userId))
        db.query(USERS_TABLE, null, "$COLUMN_ID = ?", arrayOf(userId), null, null, null)
    }
}
