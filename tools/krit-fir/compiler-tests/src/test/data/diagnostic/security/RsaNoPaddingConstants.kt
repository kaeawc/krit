// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: a transformation held by a constant. Go only reads
// a string literal argument, so it misses each call below. FIR is correct to
// report them because a `const val` (read from its evaluated value, so a
// constant concatenation counts too) and a local `val` initialized with a
// literal always hold an RSA NoPadding transformation.
package test

import javax.crypto.Cipher

private const val TOP_LEVEL = "RSA/ECB/NoPadding"
const val CONCATENATED = "RSA/" + "NONE/NoPadding"

object Transformations {
    const val NESTED = "rsa/ecb/nopadding"
}

class Crypto {
    companion object {
        private const val COMPANION = " RSA/ECB/NoPadding "
    }

    fun topLevel(): Cipher = <!RsaNoPadding!>Cipher.getInstance(TOP_LEVEL)<!>

    fun concatenated(): Cipher = <!RsaNoPadding!>Cipher.getInstance(CONCATENATED)<!>

    fun qualified(): Cipher = <!RsaNoPadding!>Cipher.getInstance(Transformations.NESTED)<!>

    fun companion(): Cipher = <!RsaNoPadding!>Cipher.getInstance(COMPANION, "BC")<!>

    fun localVal(): Cipher {
        val transformation = "RSA/ECB/NoPadding"
        return <!RsaNoPadding!>Cipher.getInstance(transformation)<!>
    }
}
