package test

import javax.crypto.Cipher

object KeyCipher {
    fun getInstance(transformation: String): String = transformation
}

class Crypto {
    fun cipher() {
        javax.crypto.Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding")
        javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding")
        Cipher.getInstance("AES/GCM/NoPadding")
    }

    fun parameter(Cipher: KeyCipher): String = Cipher.getInstance("RSA/ECB/NoPadding")

    fun localVal(): String {
        val Cipher = KeyCipher
        return Cipher.getInstance("RSA/ECB/NoPadding")
    }
}

class Keys {
    companion object Cipher {
        fun getInstance(transformation: String): String = transformation
    }

    fun companion(): String = Cipher.getInstance("RSA/ECB/NoPadding")
}
