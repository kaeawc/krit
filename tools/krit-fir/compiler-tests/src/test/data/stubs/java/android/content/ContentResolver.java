// Compiler-test source stub; never packaged in the production artifact.
package android.content;

import android.database.Cursor;
import android.net.Uri;
import java.io.FileNotFoundException;
import java.io.InputStream;

public abstract class ContentResolver {
    public ContentResolver(Context context) {
    }

    public final Cursor query(
            Uri uri,
            String[] projection,
            String selection,
            String[] selectionArgs,
            String sortOrder) {
        throw new RuntimeException("Stub!");
    }

    public final Uri insert(Uri url, ContentValues values) {
        throw new RuntimeException("Stub!");
    }

    public final int update(Uri uri, ContentValues values, String where, String[] selectionArgs) {
        throw new RuntimeException("Stub!");
    }

    public final int delete(Uri url, String where, String[] selectionArgs) {
        throw new RuntimeException("Stub!");
    }

    public final InputStream openInputStream(Uri uri) throws FileNotFoundException {
        throw new RuntimeException("Stub!");
    }

    public void notifyChange(Uri uri, Object observer) {
        throw new RuntimeException("Stub!");
    }
}
