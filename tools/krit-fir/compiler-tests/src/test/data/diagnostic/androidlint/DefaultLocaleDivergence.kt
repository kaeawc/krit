// RENDER_DIAGNOSTICS_FULL_TEXT
// Go reports every call in this file; FIR reports none. Go matches the call by
// name and skips it only when an argument mentions the identifier `Locale`.
// None of these calls uses the default locale, so the message ("Implicitly
// using the default locale") is not true of them.
package test

import java.util.Locale

class ExplicitLocales {
    // An explicit Locale held in a parameter.
    fun formatWithVariable(locale: Locale, value: Int): String = String.format(locale, "%d", value)

    // A null Locale applies no localization (java.util.Formatter), which is
    // not the default locale either.
    fun formatWithNull(value: Int): String = String.format(null, "%d", value)

    fun javaLowerWithVariable(s: String, locale: Locale): String = (s as java.lang.String).toLowerCase(locale)
}

@Suppress("DEPRECATION_ERROR")
class SuppressedExplicit {
    fun lower(s: String, locale: Locale): String = s.toLowerCase(locale)

    // Char.toLowerCase() / toUpperCase() use the invariant Unicode mapping
    // (Character.toLowerCase), never a locale.
    fun charLower(c: Char): Char = c.toLowerCase()

    fun charUpper(c: Char): Char = c.toUpperCase()
}

class CharacterStatics {
    // java.lang.Character's case mapping is locale-independent.
    fun lower(c: Char): Char = Character.toLowerCase(c)

    fun upperCodePoint(codePoint: Int): Int = Character.toUpperCase(codePoint)
}
