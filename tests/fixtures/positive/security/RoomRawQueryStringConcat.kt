package test

import androidx.sqlite.db.SimpleSQLiteQuery

class RoomRawQueryStringConcatFixture {
    fun query(term: String) {
        SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE '%$term%'")
        SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE '%" + term + "%'")
    }
}
