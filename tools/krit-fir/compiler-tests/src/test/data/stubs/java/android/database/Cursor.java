// Compiler-test source stub; never packaged in the production artifact.
package android.database;

import java.io.Closeable;

public interface Cursor extends Closeable {
    int getCount();

    boolean isClosed();

    boolean moveToFirst();

    boolean moveToNext();

    boolean moveToPosition(int position);

    int getColumnIndex(String columnName);

    int getColumnIndexOrThrow(String columnName) throws IllegalArgumentException;

    String getString(int columnIndex);

    int getInt(int columnIndex);

    long getLong(int columnIndex);

    double getDouble(int columnIndex);

    byte[] getBlob(int columnIndex);

    boolean isNull(int columnIndex);

    void close();
}
