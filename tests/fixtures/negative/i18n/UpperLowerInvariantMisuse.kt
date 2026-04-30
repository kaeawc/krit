package test

import java.util.Locale

class Sample {
    fun normalize(userName: String, email: String, currencyCode: String): String {
        val upper = userName.uppercase(Locale.ROOT)
        val lower = email.lowercase(Locale.ROOT)
        // ASCII-invariant identifier — case conversion is locale-independent.
        val code = currencyCode.uppercase()
        return upper + lower + code
    }

    // Local lookalike: a project-defined `uppercase()` with no receiver
    // should not be flagged.
    private fun uppercase(): String = "X"

    fun useLocal(): String = uppercase()
}
