package com.example.exceptions

class Service {

    fun process() {
        try {
            doWork()
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    fun dump() {
        Thread.dumpStack()
    }

    private fun doWork() {
        throw RuntimeException("failure")
    }
}
