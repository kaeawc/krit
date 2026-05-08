package com.example.exceptions

import java.io.IOException

class FileReader {

    fun readFile(path: String): String {
        try {
            return load(path)
        } catch (e: IOException) {
            throw CustomFileException("Failed to read file: $path", e)
        }
    }

    private fun load(path: String): String {
        throw IOException("file not found: $path")
    }
}

class CustomFileException(message: String, cause: Throwable) : RuntimeException(message, cause)
