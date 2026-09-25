// RENDER_DIAGNOSTICS_FULL_TEXT
// Positives and negatives for WeakMessageDigest: java.security.MessageDigest
// getInstance with a plain string literal naming a weak digest algorithm.
package test

import java.security.MessageDigest
import java.security.MessageDigest as Digest
import java.security.MessageDigest.getInstance

typealias AliasedDigest = java.security.MessageDigest

const val WEAK_ALGORITHM = "MD5"

class Crypto {
    fun weak(bytes: ByteArray) {
        <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("SHA-1")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("SHA1")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("MD2")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("MD4")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("md5")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance(" sha-1 ")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("""MD5""")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance(("SHA1"))<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("MD5", "SUN")<!>
        <!WeakMessageDigest!>java.security.MessageDigest.getInstance("MD5")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance("SHA1")<!>.digest(bytes)
        val multiline = <!WeakMessageDigest!>java.security.MessageDigest<!>
            .getInstance("MD5")
        val wrappedArgs = <!WeakMessageDigest!>MessageDigest.getInstance(<!>
            "SHA-1",
        )
        @Suppress("UNUSED_VARIABLE")
        <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>
        val inLambda = { <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!> }
        <!WeakMessageDigest!>MessageDigest.getInstance(/* weak */ "MD5")<!>
    }

    fun strong(algorithm: String) {
        MessageDigest.getInstance("SHA-256")
        MessageDigest.getInstance("SHA-384")
        MessageDigest.getInstance("SHA-512")
        MessageDigest.getInstance("SHA3-256")
        java.security.MessageDigest.getInstance("SHA-512")
    }

    fun notLiterals(algorithm: String) {
        MessageDigest.getInstance(algorithm)
        MessageDigest.getInstance(WEAK_ALGORITHM)
        MessageDigest.getInstance("MD" + "5")
        MessageDigest.getInstance("$algorithm")
        MessageDigest.getInstance("${"MD5"}")
        MessageDigest.getInstance("${ ("SHA1") }")
        MessageDigest.getInstance("MD\u0035")
        MessageDigest.getInstance("MD5\t")
    }

    fun otherSpellings() {
        Digest.getInstance("MD5")
        AliasedDigest.getInstance("MD5")
        getInstance("MD5")
    }
}
