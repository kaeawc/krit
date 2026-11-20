// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 30, 39, 46
// FIR negatives: non-verifiers and nonliteral returns, plus pinned Go-only
// true positives for literal-true returns with extra statements.
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

// Go restores Lint/main's true positive for extra statements; FIR still
// requires a single return and misses this body.
class LogsFirst : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        println(hostname)
        return true
    }
}

// Go also restores the declaration/throw cases from main. FIR's single-return
// restriction still misses these true positives.
class DeclaresFirst : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        val values = listOf(1)
        return true
    }
}

class ThrowsFirst : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        if (hostname.isEmpty()) throw IllegalArgumentException()
        return true
    }
}

class Conditional : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        if (hostname.isEmpty()) return false
        return true
    }
}

// An inline lambda can return from verify; neither engine reports this body.
class NonlocalReturn : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
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
// object is not a verifier; both Go and FIR skip it.
class AnonymousHelper : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    val probe = object {
        fun verify(first: String, second: String): Boolean = true
    }
}
