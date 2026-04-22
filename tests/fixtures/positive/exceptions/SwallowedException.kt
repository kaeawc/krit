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

    fun commentOnly() {
        try {
            doWork()
        } catch (e: Exception) {
            // ignored: e throw log handle
        }
    }

    fun messageOnlyLogging() {
        try {
            doWork()
        } catch (e: Exception) {
            logger.warn(e.message)
        }
    }

    fun nestedLambdaIgnored() {
        try {
            doWork()
        } catch (e: Exception) {
            run {
                logger.warn("failed", e)
            }
        }
    }

    private fun doWork() {
        throw RuntimeException("failure")
    }
}
