package com.example

import android.net.SSLCertificateSocketFactory

class Sockets {
    fun open() = SSLCertificateSocketFactory.getInsecure(0, null)
}
