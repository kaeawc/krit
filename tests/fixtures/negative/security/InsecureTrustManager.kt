package test

import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

class ValidatingTrustManager : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        if (chain.isNullOrEmpty()) throw CertificateException("missing chain")
    }
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("untrusted")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
