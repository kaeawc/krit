// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 16, 18, 22, 24x2, 27, 33, 40, 43, 48, 52, 56, 60, 62, 64, 66, 68, 71, 79, 81, 83
// Positives for DefaultLocale, each reported by the Go rule too: the static
// String.format without a Locale, and the no-argument String.toLowerCase() /
// toUpperCase(). K2 rejects the stdlib toLowerCase() / toUpperCase() as
// DEPRECATION_ERROR, so those calls only compile with the error suppressed;
// the java.lang.String members compile as they are. Like Go, the finding sits
// on the first line of the call expression.
package test

import java.util.Date

class Formatter {
    fun decimal(value: Double): String = <!DefaultLocale!>String.format("%.2f", value)<!>

    fun patternOnly(): String = <!DefaultLocale!>String.format("progress %% done")<!>

    fun date(): String = <!DefaultLocale!>String.format("%tF", Date())<!>

    // A Locale later in the argument list is a format argument, not the
    // locale: Go only reads the first argument.
    fun localeAsValue(): String = <!DefaultLocale!>String.format("%s", java.util.Locale.US)<!>

    fun nested(value: Int): String = <!DefaultLocale!>String.format("%s", String.format("%d", value))<!>

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
