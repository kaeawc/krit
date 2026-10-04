// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15
// A project `String.capitalize()` in the same package shadows the deprecated
// stdlib extension, a common migration shim. It converts with Locale.ROOT, so
// `name.capitalize()` below does not use the default locale and the message
// ("called without explicit Locale") is not true of it. Go matches the name
// without resolving the call and reports it; FIR does not.
package test

import java.util.Locale

fun String.capitalize(): String = replaceFirstChar { if (it.isLowerCase()) it.titlecase(Locale.ROOT) else it.toString() }

class Shim {
    fun title(name: String): String = name.capitalize()
}
