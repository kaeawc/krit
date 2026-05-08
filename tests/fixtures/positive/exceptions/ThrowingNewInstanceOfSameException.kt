package com.example.exceptions

import java.io.IOException

class FileReader {

    fun readFile(path: String): String {
        try {
            return load(path)
        } catch (e: IOException) {
            throw IOException(e)
        }
    }

    private fun load(path: String): String {
        throw IOException("file not found: $path")
    }
}
