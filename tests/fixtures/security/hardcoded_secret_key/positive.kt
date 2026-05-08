package test

import android.util.Base64
import javax.crypto.spec.SecretKeySpec

fun hardcodedSecretKeys() {
    SecretKeySpec(byteArrayOf(1, 2, 3, 4), "AES")
    SecretKeySpec("p@ssw0rd12345678".toByteArray(), "AES")
    SecretKeySpec(Base64.decode("c2VjcmV0MTIzNDU2Nzg=", Base64.DEFAULT), "AES")
}
