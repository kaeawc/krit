package com.example.exceptions

import java.io.IOException
import kotlin.coroutines.cancellation.CancellationException

class Handler {

    fun handle() {
        try {
            riskyOperation()
        } catch (e: Exception) {
            throw CustomException("Operation failed", e)
        }
    }

    // Intentional: rethrow CancellationException so the broader handler
    // below does not swallow it.
    fun handleCancellation() {
        try {
            riskyOperation()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            println("failed: $e")
        }
    }

    // Intentional: let IOException escape past the Throwable handler.
    fun handleIo() {
        try {
            riskyOperation()
        } catch (e: IOException) {
            throw e
        } catch (e: IllegalStateException) {
            println("bad state: $e")
        } catch (t: Throwable) {
            println("failed: $t")
        }
    }

    private fun riskyOperation() {
        throw RuntimeException("boom")
    }
}

class CustomException(message: String, cause: Throwable) : RuntimeException(message, cause)
