package com.example.exceptions

class FileProcessor {

    fun processFile() {
        try {
            readFile()
        } catch (e: Exception) {
            println("read failed: ${e.message}")
        } finally {
            cleanup()
        }
    }

    private fun readFile() {
        throw RuntimeException("cannot read")
    }

    private fun cleanup() {
        // safe cleanup logic
    }
}
