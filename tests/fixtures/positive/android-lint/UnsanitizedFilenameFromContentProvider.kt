package com.example

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import java.io.File

class SharedFileImporter(private val context: Context) {
    fun import(uri: Uri, cacheDir: File): File? {
        context.contentResolver.query(uri, null, null, null, null)?.use { cursor ->
            if (!cursor.moveToFirst()) return null
            val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            val displayName = cursor.getString(nameIndex)
            return File(cacheDir, displayName)
        }
        return null
    }
}
