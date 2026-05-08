package test

import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

class Crypto {
    fun keys() {
        val rsa = KeyPairGenerator.getInstance("RSA")
        rsa.initialize(1024)
        val aes = KeyGenerator.getInstance("AES")
        aes.init(64)
    }
}
