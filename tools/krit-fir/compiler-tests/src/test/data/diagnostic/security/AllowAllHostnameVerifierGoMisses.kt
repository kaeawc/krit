// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 47
// Deliberate improvements: each verify below is a HostnameVerifier's verify
// that always returns true, so each is a real finding. Go misses them because
// it only visits class declarations (not object, companion object, or
// anonymous object declarations), matches the verifier by the text
// HostnameVerifier in the class header, reports one verify per class, and
// compares the body text with `return true` or `true`; it counts the
// parameters by the commas in the text, so a trailing comma makes three.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

object AllowAllObject : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

class WithCompanion {
    companion object : HostnameVerifier {
        <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

val topLevelAnonymous: HostnameVerifier = object : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

fun anonymousInFunction(): HostnameVerifier = object : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

abstract class VerifierBase : HostnameVerifier

class DerivedAllowAll : VerifierBase() {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

typealias Verifier = HostnameVerifier

class AliasedAllowAll : Verifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

// Go reports only the class's own verify; the companion is a second verifier.
class TwoVerifiers : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true

    companion object Lenient : HostnameVerifier {
        <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

class LabeledReturn : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return@verify true
    }
}

class Parenthesized : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = (true)
}

class Backticked : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun `verify`(hostname: String, session: SSLSession): Boolean = true
}

class TrailingComma : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(
        hostname: String,
        session: SSLSession,
    ): Boolean = true
}

class ParenthesizedReturn : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return (true)
    }
}
