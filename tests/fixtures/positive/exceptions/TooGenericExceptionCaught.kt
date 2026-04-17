package com.example.exceptions

class DataService {

    fun fetchData(): String {
        try {
            return queryDatabase()
        } catch (e: Exception) {
            return "default"
        }
    }

    private fun queryDatabase(): String {
        throw RuntimeException("connection lost")
    }
}
