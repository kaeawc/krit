package com.example.exceptions

class Handler {

    fun handle() {
        try {
            riskyOperation()
        } catch (e: Exception) {
            throw e
        }
    }

    private fun riskyOperation() {
        throw RuntimeException("boom")
    }
}
