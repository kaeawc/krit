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

    // Only the caught parameter itself is checked: other values, the cause,
    // a safe cast and `when (e)` dispatch are not reported.
    fun others(other: Throwable): String {
        return try {
            loadFromNetwork()
        } catch (e: Exception) {
            val cause = e.cause
            if (other is IOException || cause is IOException || e.cause is IOException) return "io"
            val safe = e as? IOException
            when (e) {
                is IllegalStateException -> "state"
                else -> safe.toString()
            }
        }
    }

    private fun loadFromNetwork(): String = "ok"

    private fun log(e: Exception) {}
}
