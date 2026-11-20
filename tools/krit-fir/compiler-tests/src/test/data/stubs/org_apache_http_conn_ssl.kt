package org.apache.http.conn.ssl

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

open class AllowAllHostnameVerifier : HostnameVerifier {
    override fun verify(hostname: String?, session: SSLSession?): Boolean = true
}
