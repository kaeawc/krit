// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: verifiers whose verify does not always return true, and verify
// functions that are not a HostnameVerifier's.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class Validating : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
}

class Delegating(delegate: HostnameVerifier) : HostnameVerifier by delegate

class RejectAll : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = false
}

class LogsFirst : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        println(hostname)
        return true
    }
}

class Conditional : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        if (hostname.isEmpty()) return false
        return true
    }
}

class Branches : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean =
        if (hostname.isEmpty()) true else hostname == session.peerHost
}

private const val ALWAYS = true

class ConstantResult : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = ALWAYS
}

class Wrapped : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = run { true }
}

abstract class AbstractVerifier : HostnameVerifier {
    abstract override fun verify(hostname: String, session: SSLSession): Boolean
}

interface NoDefault : HostnameVerifier

// A verify(String, SSLSession) on a class that is not a HostnameVerifier.
class NotAVerifier {
    fun verify(hostname: String, session: SSLSession): Boolean = true
}

// A top-level verify is not a member of any verifier.
fun verify(hostname: String, session: SSLSession): Boolean = true

// A lambda verifier is not a declaration; Go does not report it either.
val lambdaVerifier = HostnameVerifier { _, _ -> true }
