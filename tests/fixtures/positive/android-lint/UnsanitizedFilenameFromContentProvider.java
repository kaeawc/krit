package com.example;

import android.database.Cursor;
import android.provider.OpenableColumns;
import java.io.File;

public class SharedFileImporter {
    File target(Cursor cursor, File cacheDir) {
        int nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
        String displayName = cursor.getString(nameIndex);
        return new File(cacheDir, displayName);
    }
}
