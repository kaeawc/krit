package com.example.exceptions

class FileProcessor {

    fun processFile() {
        try {
            readFile()
        } catch (e: Exception) {
            println("read failed")
        } finally {
            throw RuntimeException("cleanup failed")
        }
    }

    private fun readFile() {
        throw RuntimeException("cannot read")
    }
}
