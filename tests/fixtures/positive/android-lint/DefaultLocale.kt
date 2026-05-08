package com.example

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
