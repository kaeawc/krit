// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 31, 35, 36, 37, 39, 42, 47
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
        // A comment ending in `{` before the literal is not a template entry.
        val commentBrace = <!WeakMessageDigest!>MessageDigest.getInstance( // switch algorithm {<!>
            "MD5"
        )
        <!WeakMessageDigest!>MessageDigest.getInstance(/* { */ "SHA-1")<!>
        // Deliberate improvement: Go misses this because its parenthesis unwrap
        // reads the comment as the inner expression; FIR is correct because the
        // parenthesized value is the literal "MD5".
        <!WeakMessageDigest!>MessageDigest.getInstance(( /* { */ "MD5"))<!>
        val rawMultiline = <!WeakMessageDigest!>MessageDigest.getInstance(<!>
            """
            MD5
            """
        )
        // Deliberate improvement: Go reports neither because it reads the
        // annotation or label node as the argument; FIR is correct because the
        // argument value is still the literal "MD5".
        <!WeakMessageDigest!>MessageDigest.getInstance(@Suppress("x") "MD5")<!>
        <!WeakMessageDigest!>MessageDigest.getInstance(label@ "MD5")<!>
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
        MessageDigest.getInstance("${/* { */ "MD5"}")
        MessageDigest.getInstance(("${"MD5"}"))
        MessageDigest.getInstance("MD\u0035")
        MessageDigest.getInstance("MD5\t")
    }

    fun otherSpellings() {
        Digest.getInstance("MD5")
        AliasedDigest.getInstance("MD5")
        getInstance("MD5")
    }
}
