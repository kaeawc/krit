// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 16, 22, 28, 36, 40, 47, 52, 60, 64, 68, 74, 79, 88, 92, 98, 109
// Positive: a javax.net.ssl.HostnameVerifier whose verify(hostname, session)
// always returns true. Each is reported on the function's first line (its
// first annotation or modifier, else `fun`), the line the Go rule reports.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class AllowAllExpression : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllBlock : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return true
    }
}

class AllowAllSemicolon : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return true;
    }
}

class AllowAllCommented : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        // TODO: pin the production host
        /* accept everything for now */
        return true
    }
}

class AllowAllNullable : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String?, session: SSLSession?) = true
}

class AllowAllQualified : javax.net.ssl.HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllAnnotated : HostnameVerifier {
    /**
     * KDoc is not part of the reported line.
     */
    <!AllowAllHostnameVerifier!>@Suppress("UNUSED_PARAMETER")<!>
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllMultiline : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(
        hostname: String,
        session: SSLSession
    ): Boolean =
        true
}

open class AllowAllOpen : HostnameVerifier {
    <!AllowAllHostnameVerifier!>final<!> override fun verify(hostname: String, session: SSLSession): Boolean = true
}

abstract class AllowAllAbstract : HostnameVerifier, Cloneable {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

interface AllowAllDefault : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class AllowAllEnum : HostnameVerifier {
    INSTANCE;

    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class AllowAllEntries : HostnameVerifier {
    LENIENT {
        <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    },
    STRICT {
        override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
    },
}

class Outer {
    class NestedAllowAll : HostnameVerifier {
        <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }

    inner class InnerAllowAll : HostnameVerifier {
        <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession) = true
    }
}

fun localVerifier(): HostnameVerifier {
    class LocalAllowAll : HostnameVerifier {
        <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
    return LocalAllowAll()
}

// The anonymous object's nearest class declaration is a verifier whose own
// verify validates, so Go reaches the object's verify through it.
class Factory : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    fun lenient(): HostnameVerifier = object : HostnameVerifier {
        <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}
