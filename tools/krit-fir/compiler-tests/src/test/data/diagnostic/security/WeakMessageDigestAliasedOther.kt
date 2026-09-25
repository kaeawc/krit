// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17, 18
// Negative, deliberate precision fix: an explicit import aliases another
// java.security class to MessageDigest, which wins over the star import.
// Go reports these calls because the star import satisfies its
// java.security.MessageDigest mention check and the file declares no
// MessageDigest; FIR is correct because the receiver resolves to Signature,
// not java.security.MessageDigest. The fully qualified JDK call still fires.
package test

import java.security.*
import java.security.Signature as MessageDigest

class Crypto {
    fun hash() {
        MessageDigest.getInstance("MD5")
        MessageDigest.getInstance("SHA1")
        <!WeakMessageDigest!>java.security.MessageDigest.getInstance("MD5")<!>
    }
}
