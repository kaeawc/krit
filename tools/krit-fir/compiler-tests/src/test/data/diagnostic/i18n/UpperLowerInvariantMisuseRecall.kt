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

// Go misses this because tree-sitter's simple_identifier text keeps the
// backticks ("`uppercase`"), so it never equals "uppercase"; the call still
// resolves to kotlin.text.uppercase.
fun backtick(userName: String): String = <!UpperLowerInvariantMisuse!>userName.`uppercase`()<!>

// Go misses these because it only reports calls with an explicit receiver.
// The text that names the implicit receiver (the scope function's receiver
// argument `userName`, the extension property's name `shouted`) names no
// ASCII-invariant value, so the exemption does not apply.
fun runReceiver(userName: String): String = userName.run { <!UpperLowerInvariantMisuse!>uppercase()<!> }

fun applyReceiver(userName: String): String = userName.apply { <!UpperLowerInvariantMisuse!>uppercase()<!> }

val String.shouted: String get() = <!UpperLowerInvariantMisuse!>uppercase()<!>

// Go reports this: the explicit receiver text is `this`, which names no
// ASCII-invariant value, and FIR matches it.
fun explicitThis(currencyCode: String): String = with(currencyCode) { <!UpperLowerInvariantMisuse!>this.uppercase()<!> }
