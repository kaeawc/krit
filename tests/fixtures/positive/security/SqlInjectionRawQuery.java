package test;

import android.database.sqlite.SQLiteDatabase;

class SqlInjectionRawQueryJavaFixture {
    void load(SQLiteDatabase db, String userId) {
        db.rawQuery("SELECT * FROM users WHERE id = " + userId, null);
    }
}
