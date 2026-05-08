package com.example.exceptions

class Handler {

    fun handle() {
        try {
            riskyOperation()
        } catch (e: Exception) {
            throw CustomException("Operation failed", e)
        }
    }

    private fun riskyOperation() {
        throw RuntimeException("boom")
    }
}

class CustomException(message: String, cause: Throwable) : RuntimeException(message, cause)
