package com.example.exceptions

import java.io.IOException

class DataService {

    fun fetchData(): String {
        try {
            return queryDatabase()
        } catch (e: IOException) {
            return "default"
        }
    }

    private fun queryDatabase(): String {
        throw IOException("connection lost")
    }
}
