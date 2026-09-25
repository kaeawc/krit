// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate FIR recall additions: stdlib uppercase() / lowercase() calls
// without a Locale that Go cannot see.
package ulimrecall

import kotlin.text.lowercase as down

// Go misses these because it only reports calls with an explicit receiver
// (`x.uppercase()`); FIR resolves the implicit receiver to the stdlib call.
fun withReceiver(userName: String): String = with(userName) { <!UpperLowerInvariantMisuse!>uppercase()<!> }

fun String.normalized(): String = <!UpperLowerInvariantMisuse!>uppercase()<!>

// Go misses this because it matches the spelled name; the import alias still
// resolves to kotlin.text.lowercase (the alias hides the name `lowercase` in
// this file, so the other cases use uppercase).
fun aliased(userName: String): String = <!UpperLowerInvariantMisuse!>userName.down()<!>
