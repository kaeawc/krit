// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 15
// Same-package extensions beat the default import of kotlin.text, so these
// calls are the project functions, which take no Locale. Go reports both by
// name; FIR does not.
package test

fun String.Companion.format(pattern: String, vararg args: Any?): String = pattern + args.size

fun String.toLowerCase(): String = lowercase()

class Uses {
    fun formatted(): String = String.format("%d", 1)

    fun lower(s: String): String = s.toLowerCase()
}
