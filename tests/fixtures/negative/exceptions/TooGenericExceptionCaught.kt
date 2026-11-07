package com.example.exceptions

import java.io.IOException

class DataService {

    fun fetchData(): String {
        try {
            return queryDatabase()
        } catch (e: IOException) {
            return "default"
        }
    }

    // Names matching _|(ignore|expected).* are allowed.
    fun ignoredName() {
        try {
            queryDatabase()
        } catch (_: Exception) {
            println("ignored")
        }
        try {
            queryDatabase()
        } catch (ignoredFailure: Throwable) {
            println("ignored")
        }
    }

    // Only the exact configured names are generic; qualified names and subclasses are not matched.
    fun otherTypes() {
        try {
            queryDatabase()
        } catch (e: java.lang.Exception) {
            println(e)
        } catch (e: IllegalStateException) {
            println(e)
        }
    }

    private fun queryDatabase(): String {
        throw IOException("connection lost")
    }
}
