// RENDER_DIAGNOSTICS_FULL_TEXT
// A same-package String.uppercase() wins over the default-imported
// kotlin.text.uppercase(), so the call is the project function.
package ulimshadow

fun String.uppercase(): String = this + "!"

// Go reports this because the call is spelled `.uppercase()`; it resolves to
// the project extension above, which takes no Locale.
fun shadowed(userName: String): String = userName.uppercase()

// The unshadowed lowercase() is still the stdlib conversion.
fun unshadowed(email: String): String = <!UpperLowerInvariantMisuse!>email.lowercase()<!>
