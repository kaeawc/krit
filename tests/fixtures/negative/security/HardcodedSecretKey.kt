package test

import javax.crypto.spec.SecretKeySpec

fun runtimeSecretKey(key: ByteArray) {
    SecretKeySpec(key, "AES")
}
