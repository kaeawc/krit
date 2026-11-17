// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 21, 25, 28, 31, 37, 44, 47, 52, 56, 60, 64, 66, 68, 70, 72, 75, 83, 85, 87
// Positives for DefaultLocale, reported by the Go rule too except where noted: the static
// String.format without a Locale, and the no-argument String.toLowerCase() /
// toUpperCase(). K2 rejects the stdlib toLowerCase() / toUpperCase() as
// DEPRECATION_ERROR, so those calls only compile with the error suppressed;
// the java.lang.String members compile as they are. Like Go, the finding sits
// on the first line of the call expression.
package test

import java.util.Date

class Formatter {
    fun decimal(value: Double): String = <!DefaultLocale!>String.format("%.2f", value)<!>

    // Lint, Go and FIR skip a pattern without a locale-specific conversion
    // (d, e, E, f, g, G, t or T); this one has only `%%`
    // (DefaultLocaleLintFilters.kt).
    fun patternOnly(): String = String.format("progress %% done")

    fun date(): String = <!DefaultLocale!>String.format("%tF", Date())<!>

    // A Locale later in the argument list is a format argument, not the
    // locale: Go only reads the first argument.
    fun localeAsValue(): String = <!DefaultLocale!>String.format("%d %s", 1, java.util.Locale.US)<!>

    // The outer pattern has only `%s`.
    fun nested(value: Int): String = String.format("%s", <!DefaultLocale!>String.format("%d", value)<!>)

    fun splitReceiver(value: Int): String {
        val text = <!DefaultLocale!>String<!>
            .format("%d", value)
        return text
    }

    fun wrappedArguments(value: Int): String {
        val text = <!DefaultLocale!>String.format(<!>
            "%d",
            value,
        )
        return text
    }

    fun inLambda(values: List<Int>): List<String> = values.map { <!DefaultLocale!>String.format("%d", it)<!> }

    fun inLocalFunction(value: Int): String {
        fun local() = <!DefaultLocale!>String.format("%d", value)<!>
        return local()
    }

    fun inAnonymousObject(): Any = object {
        val text = <!DefaultLocale!>String.format("%d", 1)<!>
    }

    companion object {
        val SHARED = <!DefaultLocale!>String.format("%d", 2)<!>
    }
}

val TOP_LEVEL = <!DefaultLocale!>String.format("%d", 3)<!>

@Suppress("DEPRECATION_ERROR")
class Suppressed {
    fun lower(s: String): String = <!DefaultLocale!>s.toLowerCase()<!>

    fun upper(s: String): String = <!DefaultLocale!>s.toUpperCase()<!>

    fun safe(s: String?): String? = <!DefaultLocale!>s?.toLowerCase()<!>

    fun chained(s: String): String = <!DefaultLocale!>s.trim().toUpperCase()<!>

    fun implicitReceiver(s: String): String = with(s) { <!DefaultLocale!>toLowerCase()<!> }

    fun splitChain(s: String): String {
        val text = <!DefaultLocale!>s<!>
            .trim()
            .toLowerCase()
        return text
    }
}

class JavaMembers {
    fun lower(s: String): String = <!DefaultLocale!>(s as java.lang.String).toLowerCase()<!>

    fun upper(s: java.lang.String): String = <!DefaultLocale!>s.toUpperCase()<!>

    fun safe(s: java.lang.String?): String? = <!DefaultLocale!>s?.toUpperCase()<!>
}
