package com.example.exceptions

import java.util.logging.Logger
import java.util.logging.Level

class Processor {

    private val logger = Logger.getLogger(Processor::class.java.name)

    fun process() {
        try {
            doWork()
        } catch (e: Exception) {
            logger.log(Level.SEVERE, "Processing failed", e)
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
            logger.log(Level.SEVERE, "Failed", e)
            throw RuntimeException()
        }
    }

    fun fallbackAccumulator() {
        val rows = mutableListOf<String>()
        for (i in 0 until 10) {
            try {
                rows += transform(i)
            } catch (e: Exception) {
                rows += "*Failed to Transform*"
            }
        }
    }

    private fun transform(i: Int): String = i.toString()

    private fun doWork() {
        throw RuntimeException("failure")
    }
}
