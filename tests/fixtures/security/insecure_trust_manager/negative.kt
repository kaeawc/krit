package test

import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

class DelegatingTrustManager(delegate: X509TrustManager) : X509TrustManager by delegate

class ValidatingTrustManager : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        if (chain.isNullOrEmpty()) throw CertificateException("missing chain")
    }
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("untrusted")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

interface X509TrustManager
class LocalTrustAll : X509TrustManager {
    fun checkServerTrusted() {}
}
