// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17, 18
// Deliberate precision fix: an explicit import aliases another javax.crypto
// class to Mac, which wins over the star import. Go reports these calls
// because the star import satisfies its javax.crypto.Mac mention check and the
// file declares no Mac; FIR is correct because the receiver resolves to
// javax.crypto.KeyGenerator, not javax.crypto.Mac. The fully qualified JDK
// call still fires.
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
