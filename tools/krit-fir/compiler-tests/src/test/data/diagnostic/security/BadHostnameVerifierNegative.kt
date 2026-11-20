// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 30, 39, 46, 53, 61, 102
// Non-verifiers and nonliteral returns remain silent in FIR. Literal-true
// returns with extra statements retain main's Go/Lint findings in both engines.
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

class ExtensionHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession) = hostname == session.peerHost
    fun Any.verify(hostname: String, session: SSLSession) = true
    fun <Value> verify(hostname: Value, session: SSLSession) = true
    fun verify(hostname: String, vararg session: SSLSession) = true
}

// Both engines retain main's Go/Lint finding despite preceding logging.

class LogsFirst : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        println(hostname)
        return true
    }
}

// Declarations and throws before a final true return also retain coverage.

class DeclaresFirst : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        val values = listOf(1)
        return true
    }
}

class ThrowsFirst : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        if (hostname.isEmpty()) throw IllegalArgumentException()
        return true
    }
}

class Conditional : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        if (hostname.isEmpty()) return false
        return true
    }
}

// Main's last-return-wins quirk is retained, including inline nonlocal returns.
class NonlocalReturn : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        run { if (hostname.isEmpty()) return false }
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

// A lambda that validates the host is not an always-true verifier.
val validatingLambda = HostnameVerifier { host, _ -> host == "example.com" }

// The anonymous object's verify is not HostnameVerifier.verify, because the
// object is not a verifier; FIR skips it, while Go's class-wide visitor reports the outer member.
class AnonymousHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    val probe = object {
        fun verify(first: String, second: String): Boolean = true
    }
}
