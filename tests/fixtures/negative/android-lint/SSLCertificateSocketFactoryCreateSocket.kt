package com.example

import android.net.SSLCertificateSocketFactory

class Sockets(private val factory: SSLCertificateSocketFactory) {
    fun open(host: String) = factory.createSocket(host, 443)
}
