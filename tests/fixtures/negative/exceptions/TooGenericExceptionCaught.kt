package com.example.exceptions

import android.util.Log
import java.io.IOException
import androidx.work.CoroutineWorker
import android.content.Context
import androidx.work.WorkerParameters

class DataService {

    fun fetchData(): String {
        try {
            return queryDatabase()
        } catch (e: IOException) {
            return "default"
        }
    }

    private fun queryDatabase(): String {
        throw IOException("connection lost")
    }
}

// class MyWorker extends CoroutineWorker — a known async boundary via import.
// Catching Exception here must NOT trigger the rule.
class MyWorker(ctx: Context, params: WorkerParameters) : CoroutineWorker(ctx, params) {
    override suspend fun doWork(): Result {
        return try {
            Result.success()
        } catch (e: Exception) {
            Result.failure()
        }
    }
}

// class Runner implements Runnable — java.lang.Runnable needs no import.
// Catching Exception inside a Runnable must NOT trigger the rule.
class Runner : Runnable {
    override fun run() {
        try {
            doWork()
        } catch (e: Exception) {
            println("handled in runnable")
        }
    }

    private fun doWork() {}
}

// Caught var passed as argument to recognized logger — should not flag
fun logWithException() {
    try {
        doSomething()
    } catch (e: Exception) {
        Log.w(TAG, "msg", e)
    }
}

// Caught var rethrown in custom wrapper — should not flag
fun rethrow() {
    try {
        doSomething()
    } catch (e: Exception) {
        throw Wrapper(e)
    }
}

fun cleanupThenRethrow() {
    try {
        doSomething()
    } catch (throwable: Throwable) {
        cleanup()
        throw throwable
    }
}

private fun doSomething() {
    error("stub")
}

private fun cleanup() {}

private const val TAG = "tag"

class Wrapper(cause: Throwable) : RuntimeException(cause)
