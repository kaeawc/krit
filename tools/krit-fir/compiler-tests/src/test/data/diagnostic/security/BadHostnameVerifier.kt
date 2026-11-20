// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 17, 23, 29, 37, 41, 49, 53, 61, 65, 69, 75, 91, 95, 101, 111
// Positive: a javax.net.ssl.HostnameVerifier whose verify(hostname, session)
// always returns true. FIR reports each on the function's first line (its
// first annotation or modifier, else `fun`). Go reports the verify name,
// including the following line for the annotated declaration below.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class AllowAllExpression : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllBlock : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return true
    }
}

class AllowAllSemicolon : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        return true;
    }
}

class AllowAllCommented : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean {
        // TODO: pin the production host
        /* accept everything for now */
        return true
    }
}

class AllowAllNullable : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String?, session: SSLSession?) = true
}

class AllowAllQualified : javax.net.ssl.HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllAnnotated : HostnameVerifier {
    /**
     * KDoc is not part of the reported line.
     */
    <!BadHostnameVerifier!>@Suppress("UNUSED_PARAMETER")<!>
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllMultiline : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(
        hostname: String,
        session: SSLSession
    ): Boolean =
        true
}

open class AllowAllOpen : HostnameVerifier {
    <!BadHostnameVerifier!>final<!> override fun verify(hostname: String, session: SSLSession): Boolean = true
}

abstract class AllowAllAbstract : HostnameVerifier, Cloneable {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

interface AllowAllDefault : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class AllowAllEnum : HostnameVerifier {
    INSTANCE;

    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class AllowAllEntries : HostnameVerifier {
    // FIR inherits the enum's verifier interface for the entry body; Go does
    // not infer an entry's implicit superclass and leaves this use silent.
    LENIENT {
        <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    },
    STRICT {
        override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
    },
}

class Outer {
    class NestedAllowAll : HostnameVerifier {
        <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }

    inner class InnerAllowAll : HostnameVerifier {
        <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession) = true
    }
}

fun localVerifier(): HostnameVerifier {
    class LocalAllowAll : HostnameVerifier {
        <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
    return LocalAllowAll()
}

// The anonymous object's own supertype establishes its verifier identity.
class Factory : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost

    fun lenient(): HostnameVerifier = object : HostnameVerifier {
        <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
    }
}
