// Compiler-test source stub; never packaged in the production artifact.
package android.database.sqlite;

import android.content.Context;

public abstract class SQLiteOpenHelper implements AutoCloseable {
    public SQLiteOpenHelper(Context context, String name, SQLiteDatabase.CursorFactory factory, int version) {
    }

    public SQLiteDatabase getWritableDatabase() {
        throw new RuntimeException("Stub!");
    }

    public SQLiteDatabase getReadableDatabase() {
        throw new RuntimeException("Stub!");
    }

    public abstract void onCreate(SQLiteDatabase db);

    public abstract void onUpgrade(SQLiteDatabase db, int oldVersion, int newVersion);

    public void onOpen(SQLiteDatabase db) {
        throw new RuntimeException("Stub!");
    }

    public synchronized void close() {
        throw new RuntimeException("Stub!");
    }
}
