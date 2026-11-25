package com.example

import android.database.Cursor
import android.provider.OpenableColumns
import java.io.File

class SharedFileImporter {
    fun sanitized(cursor: Cursor, cacheDir: File): File {
        val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        val displayName = cursor.getString(nameIndex)
        val safeName = sanitize(displayName)
        return File(cacheDir, safeName)
    }

    private fun sanitize(name: String) = name.substringAfterLast('/')

    fun checked(cursor: Cursor, cacheDir: File): File? {
        val displayName = cursor.getString(cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME))
        if (displayName.contains("/")) return null
        return File(cacheDir, displayName)
    }

    fun otherColumn(cursor: Cursor, cacheDir: File): File =
        File(cacheDir, cursor.getString(cursor.getColumnIndex(OpenableColumns.SIZE)))

    fun generatedName(cursor: Cursor, cacheDir: File): File {
        val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        var displayName = cursor.getString(nameIndex)
        displayName = "import.bin"
        return File(cacheDir, displayName)
    }
}

class FakeCursor {
    fun getColumnIndex(name: String) = 0
    fun getString(index: Int) = "name"
}

class Lookalike {
    fun file(cursor: FakeCursor, cacheDir: File) =
        File(cacheDir, cursor.getString(cursor.getColumnIndex("_display_name")))
}
