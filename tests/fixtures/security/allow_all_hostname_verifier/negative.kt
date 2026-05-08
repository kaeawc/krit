package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class ValidatingVerifier : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
}

class DelegatingVerifier(delegate: HostnameVerifier) : HostnameVerifier by delegate
