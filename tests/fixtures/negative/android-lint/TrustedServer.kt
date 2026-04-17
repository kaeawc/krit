package com.example

import javax.net.ssl.SSLContext

class SecureClient {
    fun createContext(): SSLContext {
        return SSLContext.getInstance("TLS")
    }
}
