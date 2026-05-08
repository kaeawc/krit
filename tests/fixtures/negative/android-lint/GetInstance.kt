package com.example

import javax.crypto.Cipher

class CryptoHelper {
    fun encrypt(data: ByteArray): ByteArray {
        val cipher = Cipher.getInstance("AES/CBC/PKCS5Padding")
        return cipher.doFinal(data)
    }
}
