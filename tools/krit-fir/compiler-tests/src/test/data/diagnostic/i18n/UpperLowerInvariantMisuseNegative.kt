// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: explicit Locale arguments, Go's ASCII-invariant receiver
// exemption, and shapes that are not a call of the stdlib conversion.
package ulimnegative

import java.util.Locale

fun explicitLocale(userName: String, email: String, initial: Char): String {
    val upper = userName.uppercase(Locale.ROOT)
    val lower = email.lowercase(Locale.getDefault())
    val named = userName.lowercase(locale = Locale.ENGLISH)
    return upper + lower + named + initial.uppercase(Locale.ROOT)
}

class Request(val httpMethod: String, val uri: String)

// Go skips a receiver whose text contains an ASCII-invariant identifier
// (currencyCode, url, mimeType, hex, method, ...); FIR mirrors it.
fun asciiInvariant(currencyCode: String, mimeType: String?, request: Request, bytes: String): String {
    val a = currencyCode.uppercase()
    val b = mimeType?.lowercase()
    val c = request.httpMethod.uppercase()
    val d = request.uri.lowercase()
    val e = bytes.toHexString().uppercase()
    return a + b + c + d + e
}

fun String.toHexString(): String = this

// The exemption is Go's plain substring match on the receiver text, so a
// receiver that merely contains `host` (ghostName) or `url` (curlyBrace) is
// skipped as well. FIR matches Go here.
fun substringMatch(ghostName: String, curlyBrace: String): String =
    ghostName.uppercase() + curlyBrace.lowercase()

// The receiver text is the whole receiver, so an ASCII-invariant name anywhere
// in a chain exempts the call.
fun chainExempt(urls: List<String>): String = urls.first().trim().lowercase()

// Callable references are not calls.
fun references(words: List<String>): List<String> = words.map(String::uppercase)

// Other case helpers are not the rule's methods.
fun otherHelpers(initial: Char): Char = initial.uppercaseChar()
