package com.example.exceptions

class Processor {

    fun process() {
        try {
            doWork()
        } catch (e: Exception) {
            println("error occurred")
        }
    }

    fun swallowedInThrow() {
        try {
            doWork()
        } catch (e: IllegalStateException) {
            throw IllegalArgumentException(e.message)
        }
    }

    private fun doWork() {
        throw RuntimeException("failure")
    }
}
