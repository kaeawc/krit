// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 21, 26, 33, 37
// Lookalikes from nearer scopes: a member extension, a local extension and a
// local function-typed value named uppercase / lowercase all beat the
// default-imported kotlin.text functions. None is the stdlib case conversion
// and none takes a Locale, so FIR does not report them. (Explicit and star
// imports of a project `String.uppercase()` need a second file; see
// UpperLowerInvariantMisuseTest.)
package ulimscopeshadow

// Go reports `s.uppercase()` here because it matches the spelled name; the
// call resolves to the member extension `String.uppercase()` of the class.
class Member {
    fun String.uppercase(): String = this

    fun use(s: String): String = s.uppercase()
}

// Outside the class the member extension is not in scope, so the call is the
// stdlib conversion again.
fun outsideMember(s: String): String = <!UpperLowerInvariantMisuse!>s.uppercase()<!>

// Go reports `s.lowercase()` here; it resolves to the local extension.
fun localExtension(s: String): String {
    fun String.lowercase(): String = this
    return s.lowercase()
}

// Go reports `s.uppercase()` here; it is an implicit invoke() of the local
// function-typed value, not a call of kotlin.text.uppercase.
fun localFunctionValue(s: String): String {
    val uppercase: String.() -> String = { this }
    return s.uppercase()
}

// After the local scope ends, the stdlib conversion is reported again.
fun afterLocal(s: String): String = <!UpperLowerInvariantMisuse!>s.lowercase()<!>
