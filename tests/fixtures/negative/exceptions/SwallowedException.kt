package com.example.exceptions

import com.example.logging.AppLog

class Processor {

    fun process() {
        try {
            doWork()
        } catch (e: Exception) {
            AppLog.e("Processor", "Processing failed", e)
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
            AppLog.e("Processor", "Failed", e)
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
            AppLog.e("Processor", "Sink failed", error)
        }
        sink.flush()
    }

    // Passing the whole caught exception to an unrecognized function consumes
    // it — not a swallow.
    fun passedToConverter(): String {
        val out = StringBuilder()
        try {
            doWork()
        } catch (t: Throwable) {
            out.append(convertThrowableToString(t))
            return out.toString()
        }
        return "ok"
    }

    // Inspecting and dispatching on e.cause forwards the throwable chain.
    fun dispatchOnCause(): String {
        return run {
            try {
                doWork()
            } catch (e: java.util.concurrent.ExecutionException) {
                when (val cause = e.cause) {
                    is IllegalStateException -> stateError(cause)
                    else -> appError(cause)
                }
            }
        }
    }

    // A fallback assignment that ignores the exception is recovery, not a
    // swallow.
    var cached: String? = "x"

    fun fallbackAssignment() {
        try {
            doWork()
        } catch (e: Exception) {
            cached = null
        }
    }

    // An early return as a fallback value is recovery.
    fun earlyReturnRecovery(): Boolean {
        try {
            doWork()
        } catch (e: java.io.IOException) {
            return false
        }
        return true
    }

    private fun convertThrowableToString(t: Throwable): String = t.toString()

    private fun stateError(c: Throwable?): String = ""

    private fun appError(c: Throwable?): String = ""

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
