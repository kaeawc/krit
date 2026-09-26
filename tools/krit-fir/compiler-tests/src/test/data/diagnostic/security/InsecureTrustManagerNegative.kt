// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: trust managers that validate, check methods without a body, and
// lookalikes that are not trust managers. Go reports none of these either.
package test

import java.security.KeyStore
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

class Validating : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        if (chain.isNullOrEmpty()) throw CertificateException("missing chain")
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("untrusted")
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class Delegating(private val delegate: X509TrustManager) : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) =
        delegate.checkClientTrusted(chain, authType)

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        run { delegate.checkServerTrusted(chain, authType) }
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = delegate.acceptedIssuers
}

fun platformManager(): X509TrustManager {
    val factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
    factory.init(null as KeyStore?)
    val platform = factory.trustManagers.filterIsInstance<X509TrustManager>().first()
    return object : X509TrustManager {
        override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
            platform.checkClientTrusted(chain, authType)
        }

        override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
            try {
                platform.checkServerTrusted(chain, authType)
            } catch (e: CertificateException) {
                throw CertificateException("pinned", e)
            }
        }

        override fun getAcceptedIssuers(): Array<X509Certificate> = platform.acceptedIssuers
    }
}

// A check method without a body is not an implementation.
abstract class Partial : X509TrustManager {
    abstract override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?)
}

interface Declares : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?)
}

// Lookalikes: the check method names on types that are not trust managers.
interface CertificateGate {
    fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?)
}

class OpenGate : CertificateGate {
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
}

object Checks {
    fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
}

fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}

// Reading a property with a custom getter or a delegate runs code that can
// validate, so a scope function on it is not a do-nothing body. Go reports
// neither: the bodies are not empty, and Go skips a class containing " by ".
class GetterBacked : X509TrustManager {
    var current: Array<X509Certificate>? = null
    private val validated: Array<X509Certificate>
        get() = current?.takeIf { it.isNotEmpty() } ?: throw CertificateException("empty")

    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        validated.let {}
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        validated.forEach {}
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class LazyChecked : X509TrustManager {
    private val checked: String by lazy { throw CertificateException("never trusted") }

    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        checked.let {}
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        val local by lazy { chain ?: throw CertificateException("missing chain") }
        local.let {}
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

// A stdlib call off the do-nothing list may validate: `require` on a
// condition other than a null check rejects chains, and a lambda that calls
// into the certificate validates it.
class RequireFlag(private val pinned: Boolean) : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        require(pinned) {}
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        chain?.forEach { it.checkValidity() }
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

// Control flow with a branch that throws validates.
class BranchValidates(private val pinned: Boolean) : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        if (pinned) {} else throw CertificateException("unpinned")
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        when (val first = chain?.firstOrNull()) {
            null -> {}
            else -> first.checkValidity()
        }
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
