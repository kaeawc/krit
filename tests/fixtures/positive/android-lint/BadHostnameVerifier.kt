package com.example

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class AcceptAll : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class AcceptAllEnum : HostnameVerifier {
    INSTANCE;

    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

enum class MixedEntries : HostnameVerifier {
    LENIENT {
        override fun verify(hostname: String, session: SSLSession): Boolean = true
    },
    STRICT {
        override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
    },
}
