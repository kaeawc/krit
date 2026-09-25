// RENDER_DIAGNOSTICS_FULL_TEXT
// True positives Go misses: each call is the static String.format without a
// Locale, so it formats with the default locale. Go needs the receiver text to
// be exactly `String`, the call name `format`, and an unlabeled first argument
// that does not mention the identifier `Locale`.
package test

import java.util.Locale

typealias Text = String

class Recall {
    // Go misses: the receiver text is not `String`.
    fun javaStatic(value: Int): String = <!DefaultLocale!>java.lang.String.format("%d", value)<!>

    fun qualifiedKotlin(value: Int): String = <!DefaultLocale!>kotlin.String.format("%d", value)<!>

    fun companion(value: Int): String = <!DefaultLocale!>String.Companion.format("%d", value)<!>

    fun typeAlias(value: Int): String = <!DefaultLocale!>Text.format("%d", value)<!>

    // Go misses: the call has no explicit receiver.
    fun implicitReceiver(value: Int): String = with(String) { <!DefaultLocale!>format("%d", value)<!> }

    // Go misses: every argument is named, so there is no positional first
    // argument.
    fun named(value: Int): String = <!DefaultLocale!>String.format(format = "%d", args = *arrayOf(value))<!>

    // Go skips it because the pattern argument mentions `Locale`; the pattern
    // was picked for Locale.US but is still formatted in the default locale.
    fun patternMentionsLocale(patterns: Map<Locale, String>, value: Int): String =
        <!DefaultLocale!>String.format(patterns.getValue(Locale.US), value)<!>
}
