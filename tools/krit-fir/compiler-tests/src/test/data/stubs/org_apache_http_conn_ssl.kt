package org.apache.http.conn.ssl

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

open class AllowAllHostnameVerifier : HostnameVerifier {
    override fun verify(hostname: String?, session: SSLSession?): Boolean = true
}

object SSLSocketFactory {
    @JvmField val ALLOW_ALL_HOSTNAME_VERIFIER: HostnameVerifier = AllowAllHostnameVerifier()
}
