package test

import javax.crypto.Mac

class Crypto {
    fun mac() {
        Mac.getInstance("HmacMD5")
        Mac.getInstance("HmacSHA1")
    }
}
