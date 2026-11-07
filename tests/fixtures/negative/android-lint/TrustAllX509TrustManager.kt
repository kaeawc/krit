package com.example

import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

class StrictTrust : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {
        throw CertificateException("untrusted")
    }
    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {
        throw CertificateException("untrusted")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
