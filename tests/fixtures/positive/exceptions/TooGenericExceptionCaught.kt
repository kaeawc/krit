package com.example.exceptions

class DataService {

    fun fetchData(): String {
        try {
            return queryDatabase()
        } catch (e: Exception) {
            return "default"
        }
    }

    private fun queryDatabase(): String {
        throw RuntimeException("connection lost")
    }
}

// class DatabaseJob has a Job-like name but no coroutine supertype or import —
// it must NOT be treated as an async boundary; the catch still triggers.
class DatabaseJob {
    fun run() {
        try {
            doWork()
        } catch (e: Exception) {
            println("ignored")
        }
    }

    private fun doWork() {}
}

// class BackgroundTask has a Task-like name but no async supertype or import.
class BackgroundTask {
    fun execute() {
        try {
            process()
        } catch (e: Exception) {
            println("ignored")
        }
    }

    private fun process() {}
}
