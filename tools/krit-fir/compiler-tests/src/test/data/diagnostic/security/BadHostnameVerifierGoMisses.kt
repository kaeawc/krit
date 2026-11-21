// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 25, 37, 42, 50, 64
// Regression coverage for formerly missed shapes: objects, companions,
// anonymous verifiers, same-file inheritance, aliases and parenthesized returns.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

object AllowAllObject : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

class WithCompanion {
    companion object : HostnameVerifier {
        <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

val topLevelAnonymous: HostnameVerifier = object : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

fun anonymousInFunction(): HostnameVerifier = object : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

abstract class VerifierBase : HostnameVerifier

class DerivedAllowAll : VerifierBase() {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

typealias Verifier = HostnameVerifier

class AliasedAllowAll : Verifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

// Both the class and its companion are independent verifiers.
class TwoVerifiers : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true

    companion object Lenient : HostnameVerifier {
        <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

class LabeledReturn : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return@verify true
    }
}

class Parenthesized : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = (true)
}

class Backticked : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun `verify`(hostname: String, session: SSLSession): Boolean = true
}

class TrailingComma : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(
        hostname: String,
        session: SSLSession,
    ): Boolean = true
}

class ParenthesizedReturn : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return (true)
    }
}
