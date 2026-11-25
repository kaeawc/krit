package com.example;

import android.database.Cursor;
import android.provider.OpenableColumns;
import java.io.File;

public class SharedFileImporter {
    File target(Cursor cursor, File cacheDir) {
        int nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
        String displayName = cursor.getString(nameIndex);
        displayName = sanitize(displayName);
        return new File(cacheDir, displayName);
    }

    static String sanitize(String name) {
        return name.replace('/', '_');
    }

    static class FakeCursor {
        int getColumnIndex(String name) { return 0; }
        String getString(int index) { return "name"; }
    }

    File lookalike(FakeCursor cursor, File cacheDir) {
        return new File(cacheDir, cursor.getString(cursor.getColumnIndex("_display_name")));
    }
}
