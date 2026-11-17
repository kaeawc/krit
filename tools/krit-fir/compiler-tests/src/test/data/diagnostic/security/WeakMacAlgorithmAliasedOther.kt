// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Negative: an explicit import aliases another javax.crypto class to Mac,
// which wins over the star import, so the bare calls are
// javax.crypto.KeyGenerator's, not javax.crypto.Mac's (Go is silent too). The
// fully qualified JDK call still fires.
package test

import javax.crypto.*
import javax.crypto.KeyGenerator as Mac

class Crypto {
    fun hash() {
        Mac.getInstance("HmacMD5")
        Mac.getInstance("HmacSHA1")
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
    }
}
