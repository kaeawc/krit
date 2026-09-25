// RENDER_DIAGNOSTICS_FULL_TEXT
// Lookalikes: project functions named uppercase / lowercase are not the stdlib
// case conversion and take no Locale, so FIR does not report them.
package ulimlookalike

import ulimlookalike.shout as uppercase

class Casing(private val raw: String) {
    fun uppercase(): String = raw
    fun lowercase(): String = raw
}

class Tag

fun Tag.lowercase(): String = "tag"

// Go reports these because it matches `.uppercase()` / `.lowercase()` by name;
// the receiver is a project class, not a String or Char.
fun members(casing: Casing, tag: Tag): String = casing.uppercase() + casing.lowercase() + tag.lowercase()

fun String.shout(): String = this + "!"

// Go reports this because the call is spelled `.uppercase()`; the import
// alias makes it the project's String.shout(), not the stdlib conversion.
fun aliased(userName: String): String = userName.uppercase()

// A receiver-less call of a project function: neither Go nor FIR reports it.
class Local {
    private fun lowercase(): String = "x"

    fun useLocal(): String = lowercase()
}
