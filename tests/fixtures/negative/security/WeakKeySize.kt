package test

import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

class Crypto {
    fun keys(size: Int, other: Generator) {
        val rsa = KeyPairGenerator.getInstance("RSA")
        rsa.initialize(2048)
        val aes = KeyGenerator.getInstance("AES")
        aes.init(128)
        aes.init(size)
        other.initialize(1024)
    }
}

class Generator {
    fun initialize(size: Int) {}
}
