// RENDER_DIAGNOSTICS_FULL_TEXT
// Negatives for DefaultLocale that Go leaves alone too: an explicit Locale
// argument, the locale-invariant lowercase() / uppercase(), the instance
// form "%d".format(x) (not the static String.format call this rule targets;
// ImplicitDefaultLocale covers it), and callable references.
package test

import java.util.Locale

class Formatter {
    fun explicit(value: Double): String = String.format(Locale.US, "%.2f", value)

    fun defaultExplicit(value: Double): String = String.format(Locale.getDefault(), "%.2f", value)

    fun javaExplicit(value: Int): String = java.lang.String.format(Locale.ROOT, "%d", value)

    fun lower(s: String): String = s.lowercase(Locale.ROOT)

    fun lowerInvariant(s: String): String = s.lowercase()

    fun upperInvariant(s: String): String = s.uppercase()

    fun charLower(c: Char): Char = c.lowercaseChar()

    fun instance(value: Int): String = "%d".format(value)

    fun instanceWithLocale(value: Int): String = "%d".format(Locale.US, value)

    fun reference(): (String) -> String = String::lowercase

    fun javaLowerExplicit(s: String): String = (s as java.lang.String).toLowerCase(Locale.ROOT)
}

@Suppress("DEPRECATION_ERROR")
class SuppressedExplicit {
    fun lower(s: String): String = s.toLowerCase(Locale.ROOT)

    fun upper(s: String): String = s.toUpperCase(Locale.US)

    fun reference(): (String) -> String = String::toLowerCase
}
