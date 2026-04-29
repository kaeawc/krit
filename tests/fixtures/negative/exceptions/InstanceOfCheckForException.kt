package com.example.exceptions

import java.io.IOException

class ExceptionHandler {
    fun matches(other: Any): Boolean = other is ExceptionHandler
}

class NetworkClient {

    fun fetchData(): String {
        try {
            return loadFromNetwork()
        } catch (e: IOException) {
            log(e)
            return "network error"
        }
    }

    fun classify(): String {
        try {
            return loadFromNetwork()
        } catch (e: Throwable) {
            // Non-exception type whose name happens to start with "Exception"
            // must NOT trigger InstanceOfCheckForException.
            if (e is ExceptionHandler) {
                return "handled"
            }
            return "other"
        }
    }

    private fun loadFromNetwork(): String = "data"

    private fun log(e: Exception) {
        println(e.message)
    }
}
