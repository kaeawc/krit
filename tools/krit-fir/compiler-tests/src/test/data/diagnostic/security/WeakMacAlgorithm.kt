// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 27, 31, 32, 34, 36, 38, 41, 46
// Positives and negatives for WeakMacAlgorithm: javax.crypto.Mac getInstance
// with a plain string literal naming an HMAC over a broken digest.
package test

import javax.crypto.Mac

const val WEAK_MAC = "HmacMD5"

class Crypto {
    fun weak(bytes: ByteArray) {
        <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5")<!>
        <!WeakMacAlgorithm!>Mac.getInstance("HmacSHA1")<!>
        <!WeakMacAlgorithm!>Mac.getInstance("HmacMD2")<!>
        <!WeakMacAlgorithm!>Mac.getInstance("HmacSHA0")<!>
        <!WeakMacAlgorithm!>Mac.getInstance("hmacmd5")<!>
        <!WeakMacAlgorithm!>Mac.getInstance("HMACSHA1")<!>
        <!WeakMacAlgorithm!>Mac.getInstance(" HmacSHA1 ")<!>
        <!WeakMacAlgorithm!>Mac.getInstance("""HmacMD5""")<!>
        <!WeakMacAlgorithm!>Mac.getInstance(("HmacSHA1"))<!>
        <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5", "SunJCE")<!>
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
        <!WeakMacAlgorithm!>Mac.getInstance("HmacSHA1")<!>.doFinal(bytes)
        val multiline = <!WeakMacAlgorithm!>javax.crypto.Mac<!>
            .getInstance("HmacMD5")
        val wrappedArgs = <!WeakMacAlgorithm!>Mac.getInstance(<!>
            "HmacSHA1",
        )
        @Suppress("UNUSED_VARIABLE")
        <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5")<!>
        val inLambda = { <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5")<!> }
        val inObject = object {
            fun mac(): Mac = <!WeakMacAlgorithm!>Mac.getInstance("HmacSHA1")<!>
        }
        <!WeakMacAlgorithm!>Mac.getInstance(/* weak */ "HmacMD5")<!>
        // A comment ending in `{` before the literal is not a template entry.
        val commentBrace = <!WeakMacAlgorithm!>Mac.getInstance( // switch algorithm {<!>
            "HmacMD5"
        )
        <!WeakMacAlgorithm!>Mac.getInstance(/* { */ "HmacSHA1")<!>
        // Deliberate improvement: Go misses this because its parenthesis unwrap
        // reads the comment as the inner expression; FIR is correct because the
        // parenthesized value is the literal "HmacMD5".
        <!WeakMacAlgorithm!>Mac.getInstance(( /* { */ "HmacMD5"))<!>
        val rawMultiline = <!WeakMacAlgorithm!>Mac.getInstance(<!>
            """
            HmacMD5
            """
        )
        // Deliberate improvement: Go reports neither because it reads the
        // annotation or label node as the argument; FIR is correct because the
        // argument value is still the literal "HmacMD5".
        <!WeakMacAlgorithm!>Mac.getInstance(@Suppress("x") "HmacMD5")<!>
        <!WeakMacAlgorithm!>Mac.getInstance(label@ "HmacMD5")<!>
        // Deliberate improvement: Go misses this because it compares the
        // receiver text `(Mac)` with `Mac`; FIR is correct because the
        // parenthesized receiver still resolves to javax.crypto.Mac.
        <!WeakMacAlgorithm!>(Mac).getInstance("HmacMD5")<!>
        (Mac).getInstance("HmacSHA256")
    }

    fun strong() {
        Mac.getInstance("HmacSHA256")
        Mac.getInstance("HmacSHA384")
        Mac.getInstance("HmacSHA512")
        Mac.getInstance("HmacSHA3-256")
        Mac.getInstance("HmacSHA224")
        javax.crypto.Mac.getInstance("HmacSHA512")
        // Plain digest names are not HMAC algorithms.
        Mac.getInstance("MD5")
        Mac.getInstance("SHA1")
        Mac.getInstance("Hmac-SHA1")
        Mac.getInstance("HmacSHA1 extra")
    }

    fun notLiterals(algorithm: String) {
        Mac.getInstance(algorithm)
        Mac.getInstance(WEAK_MAC)
        Mac.getInstance("Hmac" + "MD5")
        Mac.getInstance("$algorithm")
        Mac.getInstance("${"HmacMD5"}")
        Mac.getInstance("${ ("HmacSHA1") }")
        Mac.getInstance("${/* { */ "HmacMD5"}")
        Mac.getInstance(("${"HmacMD5"}"))
        Mac.getInstance("HmacMD\u0035")
        Mac.getInstance("HmacMD5\t")
    }
}
