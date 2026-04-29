package com.example.exceptions

import org.signal.core.util.logging.Log

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

    fun returnedDomainFailure(): Result {
        try {
            doWork()
        } catch (e: IllegalStateException) {
            return Result.Failure(e)
        }
        return Result.Success
    }

    fun callbackFailure(callback: Callback) {
        try {
            doWork()
        } catch (e: IllegalStateException) {
            callback.onLoadFailed(e)
        }
    }

    fun callbackPropertyHandler() {
        val sink = FaultHidingSink { error ->
            Log.e("Processor", "Sink failed", error)
        }
        sink.flush()
    }

    private fun transform(i: Int): String = i.toString()

    private fun doWork() {
        throw RuntimeException("failure")
    }
}

sealed class Result {
    object Success : Result()
    data class Failure(val error: Throwable) : Result()
}

interface Callback {
    fun onLoadFailed(error: Throwable)
}

class FaultHidingSink(
    private val onException: (IllegalStateException) -> Unit,
) {
    fun flush() {
        try {
            doWork()
        } catch (e: IllegalStateException) {
            onException(e)
        }
    }

    private fun doWork() {
        throw IllegalStateException("failure")
    }
}
