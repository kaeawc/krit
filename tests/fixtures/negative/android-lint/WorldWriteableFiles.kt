package com.example

import android.content.Context

class FileHelper(private val context: Context) {
    // MODE_WORLD_WRITEABLE must never be used.
    fun openFile() = context.openFileOutput("data.txt", Context.MODE_PRIVATE)

    fun rationale(): String = "Avoid MODE_WORLD_WRITEABLE and MODE_WORLD_WRITABLE at all costs"
}
