package com.example.exceptions

import java.io.IOException

class Handler {

    fun handle() {
        try {
            riskyOperation()
        } catch (e: Exception) {
            throw e
        }
    }

    // The rethrow-only catch is the last clause: nothing after it would catch
    // the exception, so the catch is still useless.
    fun handleLast() {
        try {
            riskyOperation()
        } catch (e: IOException) {
            println("io failure: $e")
        } catch (e: Exception) {
            throw e
        }
    }

    private fun riskyOperation() {
        throw RuntimeException("boom")
    }
}
