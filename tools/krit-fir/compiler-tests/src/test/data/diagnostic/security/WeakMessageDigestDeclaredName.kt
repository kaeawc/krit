// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21
// Deliberate improvement: the file declares a nested class named
// MessageDigest, which does not shadow the java.security.MessageDigest import
// in Crypto. Go misses the bare `MessageDigest` call because it skips a bare
// receiver whenever the file declares any class, object, or typealias named
// MessageDigest; FIR is correct to report it because the call resolves to
// java.security.MessageDigest.getInstance. The fully qualified call reports in
// both.
package test

import java.security.MessageDigest

class Holder {
    class MessageDigest
}

class Crypto {
    fun hash() {
        <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>
        <!WeakMessageDigest!>java.security.MessageDigest.getInstance("MD5")<!>
    }
}
