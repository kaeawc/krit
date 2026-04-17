package com.example

import android.content.Context

class FileHelper(private val context: Context) {
    fun openFile() = context.openFileOutput("data.txt", Context.MODE_PRIVATE)
}
