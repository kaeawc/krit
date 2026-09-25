// Compiler-test source stub; never packaged in the production artifact.
package android.content;

import android.database.Cursor;
import android.net.Uri;

public abstract class ContentProvider {
    public ContentProvider() {
    }

    public final Context getContext() {
        throw new RuntimeException("Stub!");
    }

    public final Context requireContext() {
        throw new RuntimeException("Stub!");
    }

    public abstract boolean onCreate();

    public abstract Cursor query(
            Uri uri,
            String[] projection,
            String selection,
            String[] selectionArgs,
            String sortOrder);

    public abstract String getType(Uri uri);

    public abstract Uri insert(Uri uri, ContentValues values);

    public abstract int delete(Uri uri, String selection, String[] selectionArgs);

    public abstract int update(Uri uri, ContentValues values, String selection, String[] selectionArgs);
}
