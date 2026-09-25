// Compiler-test source stub; never packaged in the production artifact.
package android.database.sqlite;

import android.content.ContentValues;
import android.database.Cursor;
import android.database.SQLException;

// The framework SQLiteDatabase does NOT implement AndroidX SupportSQLiteDatabase.
public final class SQLiteDatabase extends SQLiteClosable {
    public static final int CONFLICT_REPLACE = 5;

    SQLiteDatabase() {
    }

    public static SQLiteDatabase openOrCreateDatabase(String path, CursorFactory factory) {
        throw new RuntimeException("Stub!");
    }

    public boolean isOpen() {
        throw new RuntimeException("Stub!");
    }

    public int getVersion() {
        throw new RuntimeException("Stub!");
    }

    public void execSQL(String sql) throws SQLException {
        throw new RuntimeException("Stub!");
    }

    public void execSQL(String sql, Object[] bindArgs) throws SQLException {
        throw new RuntimeException("Stub!");
    }

    public Cursor rawQuery(String sql, String[] selectionArgs) {
        throw new RuntimeException("Stub!");
    }

    public Cursor query(
            String table,
            String[] columns,
            String selection,
            String[] selectionArgs,
            String groupBy,
            String having,
            String orderBy) {
        throw new RuntimeException("Stub!");
    }

    public long insert(String table, String nullColumnHack, ContentValues values) {
        throw new RuntimeException("Stub!");
    }

    public int update(String table, ContentValues values, String whereClause, String[] whereArgs) {
        throw new RuntimeException("Stub!");
    }

    public int delete(String table, String whereClause, String[] whereArgs) {
        throw new RuntimeException("Stub!");
    }

    public void beginTransaction() {
        throw new RuntimeException("Stub!");
    }

    public void setTransactionSuccessful() {
        throw new RuntimeException("Stub!");
    }

    public void endTransaction() {
        throw new RuntimeException("Stub!");
    }

    public boolean inTransaction() {
        throw new RuntimeException("Stub!");
    }

    public static interface CursorFactory {
        Cursor newCursor(SQLiteDatabase db, Object masterQuery, String editTable, Object query);
    }
}
