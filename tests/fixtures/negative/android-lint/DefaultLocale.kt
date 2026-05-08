package com.example

import java.util.Locale

class Formatter {
    fun format(value: Double): String {
        return String.format(Locale.US, "%.2f", value)
    }

    fun lower(s: String): String {
        return s.lowercase(Locale.ROOT)
    }
}
