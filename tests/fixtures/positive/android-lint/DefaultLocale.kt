// fir-parity: skip K2 rejects the stdlib String.toLowerCase()/toUpperCase() (DEPRECATION_ERROR since Kotlin 2.1), so this fixture never compiles cleanly and Go stays authoritative for it; the FIR positives (String.format without a Locale, and the conversions under @Suppress("DEPRECATION_ERROR") or on java.lang.String) are covered by compiler-tests data
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
