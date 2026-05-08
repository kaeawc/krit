package test

import androidx.sqlite.db.SimpleSQLiteQuery

private const val USERS_TABLE = "users"
private const val COLUMN_NAME = "name"

class RoomRawQueryStringConcatSafeFixture {
    fun query(term: String) {
        SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE ?", arrayOf("%$term%"))
        SimpleSQLiteQuery("SELECT * FROM users")
        SimpleSQLiteQuery("SELECT * FROM $USERS_TABLE WHERE $COLUMN_NAME LIKE ?", arrayOf(term))
        SimpleSQLiteQuery("SELECT * FROM " + USERS_TABLE)
    }
}
