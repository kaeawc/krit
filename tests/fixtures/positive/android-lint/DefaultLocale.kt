package com.example

// The stdlib no-argument toLowerCase()/toUpperCase() are DEPRECATION_ERROR since
// Kotlin 2.1; suppressing it keeps the fixture compiling for FIR parity.
@Suppress("DEPRECATION_ERROR")
class Formatter {
    fun format(value: Double): String {
        return String.format("%.2f", value)
    }

    fun lower(s: String): String {
        return s.toLowerCase()
    }

    fun upper(s: String): String {
        return s.toUpperCase()
    }
}
