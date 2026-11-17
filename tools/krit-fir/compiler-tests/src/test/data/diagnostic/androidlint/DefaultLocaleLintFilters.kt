// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 36, 38, 42, 45, 47, 50, 52, 65
// Lint's LocaleDetector.checkFormat filters, which Go applies too: String.format
// is reported only when its pattern folds to a string with a locale-specific
// conversion, and not under a throw or as an argument of a single-letter
// android.util.Log call. Lint also reports kotlin.text's capitalize() and
// decapitalize() without a Locale; Go does not, because by name it would also
// report the locale-invariant extensions of those names that projects declare
// since the stdlib ones were deprecated (CapitalizeProject below).
package test

import android.util.Log

const val COUNT_PATTERN = "%d items"
const val PLAIN_PATTERN = "%s items"
val TOP_LEVEL_PATTERN = "%.1f%%"

@Suppress("DEPRECATION_ERROR")
class LintFilters(private val name: String) {
    private val memberPattern = "%e"
    var mutablePattern = "%d"

    // No locale-specific conversion: %s, %x, %S, %n, %% and %b print the same
    // in every locale.
    fun plain(x: Int): String = String.format("%s %x %S %n %% %b", name, x, name, true)

    fun plainConstant(x: Int): String = String.format(PLAIN_PATTERN, x)

    // A pattern that is not a constant: neither Lint nor Go can read it.
    fun parameter(pattern: String, x: Int): String = String.format(pattern, x)

    fun mutable(x: Int): String = String.format(mutablePattern, x)

    fun constant(x: Int): String = <!DefaultLocale!>String.format(COUNT_PATTERN, x)<!>

    fun topLevel(x: Double): String = <!DefaultLocale!>String.format(TOP_LEVEL_PATTERN, x)<!>

    fun member(x: Double): String = <!DefaultLocale!>String.format(memberPattern, x)<!>

    fun local(x: Int): String {
        val pattern = "%,d"
        return <!DefaultLocale!>String.format(pattern, x)<!>
    }

    fun concatenated(x: Int): String = <!DefaultLocale!>String.format("count: " + "%d", x)<!>

    fun rawString(x: Int): String = <!DefaultLocale!>String.format("""%-5d|""", x)<!>

    // `\$` is a dollar sign, so the pattern is `%1$d`.
    fun positional(x: Int): String = <!DefaultLocale!>String.format("%1\$d", x)<!>

    fun dateTime(millis: Long): String = <!DefaultLocale!>String.format("%tY", millis)<!>

    fun thrown(x: Int): Nothing = throw IllegalStateException(String.format("%d", x))

    fun thrownInLambda(x: Int): () -> Nothing = { throw IllegalStateException(listOf(String.format("%d", x)).toString()) }

    fun logged(x: Int) {
        Log.d("T", String.format("%d", x))
        Log.e("T", "value " + String.format("%d", x), null)
    }

    // The nearest call the format call is an argument of is listOf, not Log.d.
    fun loggedThroughCall(x: Int) {
        Log.d("T", listOf(<!DefaultLocale!>String.format("%d", x)<!>).toString())
    }

    // The format call is the receiver of trim(), not its argument, so the
    // nearest call it is an argument of is still Log.d.
    fun loggedThroughReceiver(x: Int) {
        Log.d("T", String.format("%d", x).trim())
    }

    fun capitalized(s: String): String = <!DefaultLocale!>s.capitalize()<!>

    fun decapitalized(s: String): String = <!DefaultLocale!>s.decapitalize()<!>

    fun capitalizedExplicit(s: String): String = s.capitalize(java.util.Locale.ROOT)
}

object CapitalizeProject {
    private fun String.capitalize(): String = replaceFirstChar { it.titlecaseChar() }

    // The project extension wins over kotlin.text's and uses no locale.
    fun project(s: String): String = s.capitalize()
}
