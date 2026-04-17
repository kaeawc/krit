package com.example.exceptions

class ResourceLoader {

    fun loadResource(): String {
        var result = ""
        try {
            result = fetchData()
        } catch (e: Exception) {
            result = "fallback"
        } finally {
            cleanup()
        }
        return result
    }

    private fun fetchData(): String = "data"

    private fun cleanup() {
        // release resources
    }
}
