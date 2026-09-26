// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: without the oracle, Go skips every "literal".format(...)
// in a file that declares any `fun String.format(` extension. The one here
// takes two Ints, so `"%d".format(value)` still calls the stdlib formatter
// with the default locale.
package test

private fun String.format(width: Int, height: Int): String = this + width + height

class UnrelatedFormatExtension {
    fun number(value: Int): String = <!ImplicitDefaultLocale!>"%d".format(value)<!>

    fun project(): String = "%d".format(1, 2)
}
