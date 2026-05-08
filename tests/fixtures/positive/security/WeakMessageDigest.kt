package test

import java.security.MessageDigest

class Crypto {
    fun hash() {
        MessageDigest.getInstance("MD5")
        MessageDigest.getInstance("SHA-1")
    }
}
