package test

import java.security.SecureRandom

class Crypto {
    private val field = ByteArray(16)
    fun params(param: ByteArray) {
        val random = ByteArray(16)
        SecureRandom().nextBytes(random)
        javax.crypto.spec.IvParameterSpec(param)
        javax.crypto.spec.IvParameterSpec(field)
        javax.crypto.spec.IvParameterSpec(random)
    }
}
