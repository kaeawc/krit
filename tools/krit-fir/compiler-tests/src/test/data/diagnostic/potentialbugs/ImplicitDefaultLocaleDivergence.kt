// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 17, 21, 24, 27, 31, 38, 40, 53, 55, 57
// Go reports every call in this file; FIR reports none. Go matches the call by
// name without resolving it and treats a Locale as explicit only when the
// first argument is spelled `Locale.` or `Locale(`. None of these calls
// formats or case-converts text with the default locale, so the message
// ("called without explicit Locale" / "uses implicit default locale") is not
// true of them.
package test

import java.util.Locale

class ExplicitLocales {
    // An explicit Locale held in a parameter.
    fun staticWithVariable(locale: Locale, value: Int): String = String.format(locale, "%d", value)

    fun instanceWithVariable(locale: Locale, value: Int): String = "%d".format(locale, value)

    // A null Locale applies no localization (java.util.Formatter), which is
    // not the default locale either.
    fun staticWithNull(value: Int): String = String.format(null, "%d", value)

    // The instance form of the same null Locale: `format(locale: Locale?, ...)`.
    fun instanceWithNull(value: Int): String = "%d".format(null, value)

    // A fully qualified Locale: Go only accepts the text `Locale.` / `Locale(`.
    fun staticQualifiedLocale(value: Int): String = String.format(java.util.Locale.US, "%d", value)

    // Named arguments in another order still select the Locale overload; Go
    // reads the first argument as written, `format = "%d"`.
    fun staticNamedReordered(value: Int): String = String.format(format = "%d", locale = Locale.US, args = *arrayOf(value))
}

@Suppress("DEPRECATION_ERROR")
class CharConversions {
    // Char.toLowerCase() / toUpperCase() use the invariant Unicode mapping
    // (Character.toLowerCase), never a locale.
    fun lower(c: Char): Char = c.toLowerCase()

    fun upper(c: Char): Char = c.toUpperCase()
}

// Project declarations that share the stdlib names.
class Name(private val raw: String) {
    fun toLowerCase(): String = raw

    fun capitalize(): Name = this
}

fun Int.decapitalize(): Int = this

class Lookalikes {
    fun member(name: Name): String = name.toLowerCase()

    fun memberCapitalize(name: Name): Name = name.capitalize()

    fun extension(value: Int): Int = value.decapitalize()
}
