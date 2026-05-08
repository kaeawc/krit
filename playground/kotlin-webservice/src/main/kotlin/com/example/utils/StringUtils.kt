package com.example.utils

// UtilityClassWithPublicConstructor — class has only static-like members
// but has an implicit public constructor
class StringUtils {

    companion object {
        // MagicNumber
        private val MAX_LENGTH = 255
        private val TRUNCATE_SUFFIX_LENGTH = 3

        fun capitalize(input: String): String {
            if (input.isEmpty()) return input
            return input[0].uppercaseChar() + input.substring(1)
        }

        fun truncate(input: String, maxLen: Int): String {
            if (input.length <= maxLen) return input
            return input.substring(0, maxLen - 3) + "..."
        }

        fun isValidEmail(email: String): Boolean {
            // MagicNumber in regex length check
            return email.contains("@") && email.length > 5 && email.length < 254
        }

        fun slugify(input: String): String {
            return input.lowercase()
                .replace(Regex("[^a-z0-9\\s-]"), "")
                .replace(Regex("[\\s-]+"), "-")
                .trim('-')
        }

        // UnusedParameter: 'locale' is never read
        fun padLeft(input: String, length: Int, padChar: Char = ' ', locale: String = "en"): String {
            if (input.length >= length) return input
            return padChar.toString().repeat(length - input.length) + input
        }

        // ForEachOnRange could be replaced with repeat
        fun repeat(input: String, times: Int): String {
            val sb = StringBuilder()
            for (i in 1..times) {
                sb.append(input)
            }
            return sb.toString()
        }
    }
}
