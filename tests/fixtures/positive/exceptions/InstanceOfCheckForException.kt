package com.example.exceptions

import java.io.IOException

class ErrorClassifier {

    // Checking the type of the caught parameter in the catch body is what
    // multiple catch blocks are for.
    fun classify(): String {
        try {
            return "ok"
        } catch (e: Exception) {
            if (e is IOException) {
                return "io"
            }
            return "other"
        }
    }
}
