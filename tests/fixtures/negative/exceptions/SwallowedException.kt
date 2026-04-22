package com.example.exceptions

class Processor {

    fun process() {
        try {
            doWork()
        } catch (e: Exception) {
            Log.e("Processor", "Processing failed", e)
        }
    }

    fun wrappedException() {
        try {
            doWork()
        } catch (e: IllegalStateException) {
            throw IllegalArgumentException(e)
        }
    }

    fun wrappedWithMessage() {
        try {
            doWork()
        } catch (e: IllegalStateException) {
            throw IllegalArgumentException(e.message, e)
        }
    }

    fun ignoredName() {
        try {
            doWork()
        } catch (_: Exception) {
            println("ignored")
        }
    }

    fun loggedAndThrowNew() {
        try {
            doWork()
        } catch (e: Exception) {
            Log.e("Processor", "Failed", e)
            throw RuntimeException()
        }
    }

    fun fallbackAccumulator() {
        val rows = mutableListOf<String>()
        for (i in 0 until 10) {
            try {
                rows += transform(i)
            } catch (e: Exception) {
                rows += "*Failed to Transform*: ${e.message}"
            }
        }
    }

    private fun transform(i: Int): String = i.toString()

    private fun doWork() {
        throw RuntimeException("failure")
    }
}
