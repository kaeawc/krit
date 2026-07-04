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

    // Narrowing an ALREADY-CAUGHT exception (the catch parameter) with `is`
    // to decide rethrow-vs-wrap is the idiomatic, legitimate use — not the
    // "type-check instead of polymorphism" smell. Must NOT fire.
    fun rethrowOrWrap() {
        try {
            loadFromNetwork()
        } catch (e: IllegalStateException) {
            if (e is java.io.UncheckedIOException) {
                throw RuntimeException(e)
            } else {
                throw e
            }
        }
    }

    // `when (e) { is X -> ... }` dispatch on the caught variable is the
    // idiomatic way to group related exception types. Must NOT fire.
    fun whenDispatch(): String {
        return try {
            loadFromNetwork()
        } catch (e: Exception) {
            when (e) {
                is IOException -> "io"
                is IllegalStateException -> "state"
                else -> "other"
            }
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
