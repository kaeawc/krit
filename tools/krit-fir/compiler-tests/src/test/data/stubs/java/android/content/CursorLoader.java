// Compiler-test source stub; never packaged in the production artifact.
package android.content;

import android.database.Cursor;
import android.net.Uri;

// Real chain: CursorLoader -> AsyncTaskLoader<Cursor> -> Loader<Cursor>, not modeled.
@Deprecated
public class CursorLoader {
    public CursorLoader(Context context) {
    }

    public CursorLoader(
            Context context,
            Uri uri,
            String[] projection,
            String selection,
            String[] selectionArgs,
            String sortOrder) {
    }

    public Cursor loadInBackground() {
        throw new RuntimeException("Stub!");
    }
}
