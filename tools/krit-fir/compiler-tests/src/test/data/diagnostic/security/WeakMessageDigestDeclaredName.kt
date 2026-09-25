// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18
// A file that declares any class named MessageDigest (here a nested class that
// does not shadow the import) keeps the bare `MessageDigest` receiver silent,
// matching the Go rule's same-file lookalike guard. The fully qualified call
// is still flagged.
package test

import java.security.MessageDigest

class Holder {
    class MessageDigest
}

class Crypto {
    fun hash() {
        MessageDigest.getInstance("MD5")
        <!WeakMessageDigest!>java.security.MessageDigest.getInstance("MD5")<!>
    }
}
