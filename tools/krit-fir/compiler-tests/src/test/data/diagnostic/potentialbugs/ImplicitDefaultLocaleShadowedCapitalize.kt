// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A project `String.capitalize()` in the same package shadows the deprecated
// stdlib extension, a common migration shim. It converts with Locale.ROOT, so
// `name.capitalize()` below does not use the default locale and the message
// ("called without explicit Locale") is not true of it. Neither engine
// reports it: FIR resolves the call to the shim, and Go sees the String
// extension declared in the file (an imported shim works the same way).
package test

import java.util.Locale

fun String.capitalize(): String = replaceFirstChar { if (it.isLowerCase()) it.titlecase(Locale.ROOT) else it.toString() }

class Shim {
    fun title(name: String): String = name.capitalize()
}
