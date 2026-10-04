// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Positive: a star import of java.security resolves the bare MessageDigest
// receiver, and a companion object named MessageDigest elsewhere in the file
// does not count as a lookalike declaration.
package test

import java.security.*

class Registry {
    companion object MessageDigest
}

class Crypto {
    fun hash() {
        <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>
        MessageDigest.getInstance("SHA-256")
    }
}
