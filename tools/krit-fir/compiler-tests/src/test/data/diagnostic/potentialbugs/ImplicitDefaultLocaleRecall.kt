// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: every call below formats or case-converts with the
// default locale.
package test

import java.util.Locale
import kotlin.text.toLowerCase as lower

typealias Text = String

class OtherSpellings {
    // Go only recognises the receiver text `String`.
    fun qualifiedKotlin(value: Int): String = <!ImplicitDefaultLocale!>kotlin.String.format("%d", value)<!>

    @Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
    fun javaStatic(value: Int): String = <!ImplicitDefaultLocale!>java.lang.String.format("%d", value)<!>

    fun typeAlias(value: Int): String = <!ImplicitDefaultLocale!>Text.format("%d", value)<!>

    // The companion spelled out, `this` bound to it, or held in a variable:
    // each still calls the stdlib `String.Companion.format` without a Locale.
    fun companion(value: Int): String = <!ImplicitDefaultLocale!>String.Companion.format("%d", value)<!>

    fun companionThis(value: Int): String = with(String) { <!ImplicitDefaultLocale!>this.format("%d", value)<!> }

    fun companionVariable(value: Int): String {
        val c = String
        return <!ImplicitDefaultLocale!>c.format("%d", value)<!>
    }

    // Go matches the method name as written.
    @Suppress("DEPRECATION_ERROR")
    fun importAlias(s: String): String = <!ImplicitDefaultLocale!>s.lower()<!>
}

class LocaleMentionedInPattern {
    // Go skips a call whose first argument starts with `Locale.`; here it is
    // the pattern or a format argument, and the overload without a Locale is
    // called.
    fun staticPattern(value: Int): String = <!ImplicitDefaultLocale!>String.format(Locale.US.toString(), value)<!>

    fun instanceArgument(): String = <!ImplicitDefaultLocale!>"%d".format(Locale.US.hashCode())<!>
}
