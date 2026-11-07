// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 28
// True positives: each call is the static String.format without a Locale, so
// it formats with the default locale. Go matches the call by name: the receiver
// text must be `String` or `java.lang.String`, and the first positional (or
// `locale`-labeled) argument must not be a Locale. It reads the lines listed in
// go-lines and misses the rest.
package test

import java.util.Locale

typealias Text = String

class Recall {
    // Go reports `java.lang.String.format`; it misses the other receiver spellings.
    fun javaStatic(value: Int): String = <!DefaultLocale!>java.lang.String.format("%d", value)<!>

    fun qualifiedKotlin(value: Int): String = <!DefaultLocale!>kotlin.String.format("%d", value)<!>

    fun companion(value: Int): String = <!DefaultLocale!>String.Companion.format("%d", value)<!>

    fun typeAlias(value: Int): String = <!DefaultLocale!>Text.format("%d", value)<!>

    // Go misses: the call has no explicit receiver.
    fun implicitReceiver(value: Int): String = with(String) { <!DefaultLocale!>format("%d", value)<!> }

    // Go reports it: no positional first argument holds a Locale.
    fun named(value: Int): String = <!DefaultLocale!>String.format(format = "%d", args = *arrayOf(value))<!>

    // Go skips it because the pattern argument mentions `Locale`; the pattern
    // was picked for Locale.US but is still formatted in the default locale.
    fun patternMentionsLocale(patterns: Map<Locale, String>, value: Int): String =
        <!DefaultLocale!>String.format(patterns.getValue(Locale.US), value)<!>
}

class CompanionExpressions {
    // Go misses: the receiver is an expression of type String.Companion whose
    // text is not `String`; it is the same static default-locale format.
    fun fromLambda(value: Int): String = <!DefaultLocale!>value.let { String }.format("%d", value)<!>

    fun fromVariable(value: Int): String {
        val c = String.Companion
        return <!DefaultLocale!>c.format("%d", value)<!>
    }
}
