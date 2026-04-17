package com.example.exceptions

import java.io.IOException

class NetworkClient {

    fun fetchData(): String {
        try {
            return loadFromNetwork()
        } catch (e: Exception) {
            if (e is IOException) {
                return "network error"
            }
            return "unknown error"
        }
    }

    private fun loadFromNetwork(): String = "data"
}
