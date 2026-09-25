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

// Deliberate bypass of Go's ASCII-invariant exemption. Go skips
// `currencyCode.uppercase()` because the receiver text names an ASCII-only
// value, but it never sees these implicit-receiver calls at all. FIR has no
// receiver text to test, so it reports them: the stdlib uppercase() is called
// without a Locale, which is what the message says.
fun withInvariant(currencyCode: String): String = with(currencyCode) { <!UpperLowerInvariantMisuse!>uppercase()<!> }

fun runInvariant(currencyCode: String): String = currencyCode.run { <!UpperLowerInvariantMisuse!>uppercase()<!> }

fun String.hexUpper(): String = <!UpperLowerInvariantMisuse!>uppercase()<!>
