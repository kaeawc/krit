package test

import javax.crypto.spec.SecretKeySpec

fun hardcodedSecretKey() {
    SecretKeySpec("p@ssw0rd12345678".toByteArray(), "AES")
}
