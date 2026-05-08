package com.example

import java.security.KeyStore

class CryptoHelper {
    fun getKey(): java.security.PrivateKey {
        return loadKeyFromKeyStore()
    }
}
