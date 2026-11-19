// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Negative: an explicit import aliases another java.security class to
// MessageDigest, which wins over the star import, so the bare calls are
// Signature's, not java.security.MessageDigest's (Go is silent too). The fully
// qualified JDK call still fires.
package test

import java.security.*
import java.security.Signature as MessageDigest

class Crypto {
    fun digest() {
        MessageDigest.getInstance("MD5")
        MessageDigest.getInstance("SHA1")
        <!WeakMessageDigest!>java.security.MessageDigest.getInstance("MD5")<!>
    }
}
