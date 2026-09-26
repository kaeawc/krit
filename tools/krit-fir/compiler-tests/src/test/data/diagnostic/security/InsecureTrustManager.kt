// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 15, 20, 24, 34, 48, 49, 55, 64, 65, 72, 78, 91, 101, 115, 123, 124
// Positive: a trust manager's checkServerTrusted / checkClientTrusted whose
// body does nothing. Each is reported once, on the function's first line (its
// modifier list, else `fun`), the line the Go rule reports.
package test

import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.TrustManager
import javax.net.ssl.X509TrustManager

class TrustAll : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class CommentedOut : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        // intentionally empty
    }

    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        /* TODO: validate */
        return
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class Annotated : X509TrustManager {
    /** KDoc is not part of the Go node, so the annotation's line is reported. */
    <!InsecureTrustManager!>@Throws(CertificateException::class)<!>
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
    }

    @Throws(CertificateException::class)
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        if (chain.isNullOrEmpty()) throw CertificateException("empty")
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class Holder {
    val manager: TrustManager = object : X509TrustManager {
        <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
        <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
        override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
    }
}

val topLevelManager = object : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        chain?.forEach { it.checkValidity() }
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

fun localManager(): X509TrustManager {
    class Local : X509TrustManager {
        <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
        <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
        override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
    }
    return Local()
}

interface DefaultTrust : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
}

abstract class Overloads : X509TrustManager {
    // Like Go, the parameters are not inspected: an overload declared on a
    // trust manager with the check method's name counts.
    <!InsecureTrustManager!>fun<!> checkServerTrusted(chain: List<X509Certificate>) {}
}

class Outer : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no clients")
    }
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()

    companion object : X509TrustManager {
        <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
        override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
            throw CertificateException("no servers")
        }
        override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
    }
}

enum class Modes : X509TrustManager {
    LAX {
        <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    };

    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no clients")
    }
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class RunBody : X509TrustManager {
    // Go's brace scan finds the empty lambda; the body does nothing.
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = run {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class ScopeBodies : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = chain.let {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) = with(chain) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
