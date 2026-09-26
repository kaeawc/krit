// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24
// Negative: a file-local Mac class with a getInstance factory is not
// javax.crypto.Mac, so a weak algorithm name passed to it must NOT trigger
// WeakMacAlgorithm (Go is silent too). The fully qualified JDK call still
// does.
package test

class Mac {
    companion object {
        fun getInstance(name: String): Mac = Mac()
    }
}

object Hmac {
    fun getInstance(name: String): String = name
}

class Crypto {
    fun hash() {
        Mac.getInstance("HmacMD5")
        Mac.getInstance("HmacSHA1")
        Hmac.getInstance("HmacMD5")
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
    }
}
