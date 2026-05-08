package test;

import android.database.sqlite.SQLiteDatabase;

class SqlInjectionRawQueryJavaSafeFixture {
    static final String USERS_TABLE = "users";

    void load(SQLiteDatabase db, String userId) {
        db.rawQuery("SELECT * FROM users WHERE id = ?", new String[] { userId });
        db.rawQuery("SELECT * FROM " + USERS_TABLE + " WHERE id = ?", new String[] { userId });
    }
}
