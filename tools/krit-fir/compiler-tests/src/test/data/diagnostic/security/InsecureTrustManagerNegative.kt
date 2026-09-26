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
