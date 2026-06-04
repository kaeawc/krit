package com.example.exceptions

import java.io.IOException

class ErrorClassifier {

    // Type-checking an exception value that is NOT a caught variable to
    // branch on its concrete type is the "instanceof instead of
    // polymorphism" smell this rule targets.
    fun classify(failure: Throwable): String {
        try {
            return "ok"
        } catch (outer: Exception) {
            // `failure` is a parameter, not the caught variable `outer`.
            if (failure is IOException) {
                return "io"
            }
            return "other"
        }
    }
}
