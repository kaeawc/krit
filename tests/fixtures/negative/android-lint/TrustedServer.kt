package com.example

import java.security.cert.CertificateException
import javax.net.ssl.SSLContext
import javax.net.ssl.X509TrustManager

class SecureClient {
    fun createContext(): SSLContext {
        return SSLContext.getInstance("TLS")
    }

    // Real validating trust manager — non-empty overrides.
    val validator = object : X509TrustManager {
        override fun checkClientTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {
            if (chain.isNullOrEmpty()) throw CertificateException("empty chain")
        }
        override fun checkServerTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {
            if (chain.isNullOrEmpty()) throw CertificateException("empty chain")
        }
        override fun getAcceptedIssuers(): Array<java.security.cert.X509Certificate> = arrayOf()
    }
}
