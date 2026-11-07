package com.example

import android.net.SSLCertificateSocketFactory
import java.net.InetAddress

class Sockets(private val factory: SSLCertificateSocketFactory) {
    fun open(address: InetAddress) = factory.createSocket(address, 443)
}
