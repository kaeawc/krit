package com.example

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class AcceptAll : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}
