// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go's ASCII-invariant exemption is a user opt-out: `currencyCode.uppercase()`
// is not reported because the receiver text names an ASCII-only value. Go
// never sees these implicit-receiver calls; FIR applies the same exemption to
// the text that names the implicit receiver, so none of them is reported.
package ulimimplicit

// The receiver argument of a stdlib scope function: `currencyCode` for
// `with(currencyCode)`, `currencyCode.run`, and `currencyCode.apply`.
fun withInvariant(currencyCode: String): String = with(currencyCode) { uppercase() }

fun runInvariant(currencyCode: String): String = currencyCode.run { uppercase() }

fun applyInvariant(request: Request): String = request.mimeType.apply { lowercase() }

class Request(val mimeType: String)

// A smart cast to String keeps the receiver argument.
fun smartCastInvariant(currencyCode: Any): String =
    with(currencyCode) { if (this is String) uppercase() else "" }

// An extension function or property: its receiver is named by its label,
// the declaration's own name (`this@hexUpper`).
fun String.hexUpper(): String = uppercase()

val String.urlLower: String get() = lowercase()

// Nothing names the receiver of a lambda that is not a stdlib scope
// function's, or of a scope function called on an implicit receiver, so the
// exemption cannot be tested and FIR does not report these calls.
fun String.transform(block: String.() -> String): String = block()

fun otherLambda(userName: String): String = userName.transform { uppercase() }

fun String.nested(): String = run { apply { lowercase() } }

// The exemption still reads the scope function's receiver text, not the
// value: `userName` names no ASCII-invariant value, so this is reported.
fun notInvariant(userName: String): String = with(userName) { <!UpperLowerInvariantMisuse!>uppercase()<!> }
