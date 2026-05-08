package test

import java.security.KeyStore
import javax.crypto.SecretKey
import javax.crypto.spec.SecretKeySpec

class SecretKeySpec(bytes: ByteArray, algorithm: String)

fun runtimeSecretKeys(keyStore: KeyStore, param: ByteArray, runtime: ByteArray) {
    javax.crypto.spec.SecretKeySpec(param, "AES")
    javax.crypto.spec.SecretKeySpec(runtime, "AES")
    keyStore.getKey("alias", null) as SecretKey
    SecretKeySpec(byteArrayOf(1, 2, 3, 4), "AES")
}
