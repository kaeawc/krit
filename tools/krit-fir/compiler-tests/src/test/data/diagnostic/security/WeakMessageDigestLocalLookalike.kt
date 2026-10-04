// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18
// Negative: a file-local MessageDigest class with a getInstance factory is not
// java.security.MessageDigest, so a weak algorithm name passed to it must NOT
// trigger WeakMessageDigest. The fully qualified JDK call still does.
package test

class MessageDigest {
    companion object {
        fun getInstance(name: String): MessageDigest = MessageDigest()
    }
}

class Crypto {
    fun hash() {
        MessageDigest.getInstance("MD5")
        MessageDigest.getInstance("SHA-1")
        <!WeakMessageDigest!>java.security.MessageDigest.getInstance("MD5")<!>
    }
}
