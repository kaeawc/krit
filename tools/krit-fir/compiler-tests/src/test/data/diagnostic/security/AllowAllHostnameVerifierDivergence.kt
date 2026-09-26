// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 26, 32, 41, 52, 62
// Go findings FIR drops: each function below is not HostnameVerifier.verify,
// or does not always return true, so the message is false of the code. Go
// takes any function named verify with two parameters whose nearest enclosing
// class declaration has HostnameVerifier in its header, reads an expression
// body as the text after its last `=`, and reports only the first such
// function per class.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

// Go reports the verify(Int, Int) overload, the first two-parameter verify
// returning true in the class, and so misses the real override after it.
// FIR reports the override only.
class OverloadFirst : HostnameVerifier {
    fun verify(first: Int, second: Int): Boolean = true

    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

// Go reports this because the header mentions HostnameVerifier (a
// constructor parameter); the class is not a verifier.
class Holder(val delegate: HostnameVerifier) {
    fun verify(hostname: String, session: SSLSession): Boolean = true
}

// Go reports this because the text after the last `=` is `true`; the body
// compares the hostname and does not always return true.
class ComparesToTrue : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname.isEmpty() == true
}

// Go reports the local function because its nearest class declaration is the
// verifier; it is not the verifier's verify.
class LocalFunction : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    fun check(host: String): Boolean {
        fun verify(hostname: String, session: SSLSession): Boolean = true
        return host.isNotEmpty()
    }
}

// Go reports the companion's verify: companion objects are not class
// declarations, so its nearest class declaration is the verifier.
class CompanionHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    companion object {
        fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}

// Go reports the anonymous object's verify for the same reason; the object
// is not a verifier.
class AnonymousHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    val probe = object {
        fun verify(first: String, second: String): Boolean = true
    }
}
