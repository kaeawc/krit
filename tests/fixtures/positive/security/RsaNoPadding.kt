package test

import javax.crypto.Cipher

class Crypto {
    fun cipher() {
        Cipher.getInstance("RSA/ECB/NoPadding")
        Cipher.getInstance("RSA/NONE/NoPadding")
    }
}
