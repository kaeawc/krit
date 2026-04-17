package com.example

import android.os.Environment

class FileHelper {
    fun getDownloadPath(): String {
        val path = Environment.getExternalStorageDirectory().absolutePath
        return path
    }
}
