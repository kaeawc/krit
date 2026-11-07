package com.example

import android.net.SSLCertificateSocketFactory

class Sockets {
    fun open() = SSLCertificateSocketFactory.getDefault(0, null)
}
