package com.example

import android.content.Context

class FileHelper(private val context: Context) {
    // MODE_WORLD_WRITEABLE must never be used.
    private val rationale = "Avoid MODE_WORLD_WRITEABLE and MODE_WORLD_WRITABLE at all costs"

    fun openFile() = context.openFileOutput("data.txt", Context.MODE_PRIVATE)
}
