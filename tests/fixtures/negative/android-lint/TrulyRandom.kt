package test

import javax.crypto.Cipher

fun createCipher(): Cipher {
    return Cipher.getInstance("AES")
}
