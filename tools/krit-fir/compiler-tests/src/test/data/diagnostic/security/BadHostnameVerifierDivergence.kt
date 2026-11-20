// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Former false positives now excluded by both engines: wrong overloads,
// constructor-parameter mentions, comparisons, local and companion helpers.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

// Both engines report the actual override, not the overload before it.
class OverloadFirst : HostnameVerifier {
    fun verify(first: Int, second: Int): Boolean = true

    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

// A constructor parameter does not establish verifier inheritance.
class Holder(val delegate: HostnameVerifier) {
    fun verify(hostname: String, session: SSLSession): Boolean = true
}

// A comparison against true is not a literal-true body.
class ComparesToTrue : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname.isEmpty() == true
}

// A local function is not a verifier member.
class LocalFunction : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    fun check(host: String): Boolean {
        fun verify(hostname: String, session: SSLSession): Boolean = true
        return host.isNotEmpty()
    }
}

// A companion that does not implement HostnameVerifier is not a verifier.
class CompanionHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    companion object {
        fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

// The generic overload is not HostnameVerifier.verify.
class GenericOverloadFirst : HostnameVerifier {
    fun <T> verify(first: T, session: SSLSession): Boolean = true

    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
}
