// Compiler-test source stub; never packaged in the production artifact.
package android.database.sqlite;

import java.io.Closeable;

public abstract class SQLiteClosable implements Closeable {
    public SQLiteClosable() {
    }

    public void close() {
        throw new RuntimeException("Stub!");
    }
}
