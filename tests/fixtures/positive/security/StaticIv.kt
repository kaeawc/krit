package test

import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec

class Crypto {
    fun params() {
        IvParameterSpec(byteArrayOf(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))
        GCMParameterSpec(128, "000000000000".toByteArray())
    }
}
