package com.example.exceptions

import java.util.logging.Logger

class Service {

    private val logger = Logger.getLogger(Service::class.java.name)

    fun process() {
        try {
            doWork()
        } catch (e: Exception) {
            logger.severe("Operation failed: ${e.message}")
        }
    }

    private fun doWork() {
        throw RuntimeException("failure")
    }
}
