// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: padded RSA, non-RSA NoPadding, malformed transformations,
// non-literal arguments, and local getInstance lookalikes must NOT trigger
// RsaNoPadding. (Aliased spellings of javax.crypto.Cipher are positives, in
// RsaNoPaddingSpellings.kt.)
package test

import javax.crypto.Cipher

const val RSA_NO_PADDING = "RSA/ECB/NoPadding"

object KeyCipher {
    fun getInstance(transformation: String): String = transformation
}

class Crypto(private val mode: String) {
    fun oaep(): Cipher = Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding")

    fun pkcs1(): Cipher = Cipher.getInstance("RSA/ECB/PKCS1Padding")

    fun aesNoPadding(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")

    fun rsaOnly(): Cipher = Cipher.getInstance("RSA")

    fun twoParts(): Cipher = Cipher.getInstance("RSA/NoPadding")

    fun emptyMode(): Cipher = Cipher.getInstance("RSA//NoPadding")

    fun fourParts(): Cipher = Cipher.getInstance("RSA/ECB/NoPadding/Extra")

    // Go only reads a plain string literal as the transformation.
    fun interpolated(): Cipher = Cipher.getInstance("RSA/$mode/NoPadding")

    fun concatenated(): Cipher = Cipher.getInstance("RSA/ECB/" + "NoPadding")

    fun constant(): Cipher = Cipher.getInstance(RSA_NO_PADDING)

    fun variable(): Cipher {
        val transformation = "RSA/ECB/NoPadding"
        return Cipher.getInstance(transformation)
    }

    // Local lookalikes: same method name, different owner.
    fun lookalikeObject(): String = KeyCipher.getInstance("RSA/ECB/NoPadding")

    fun getInstance(transformation: String): String = transformation

    fun lookalikeMember(): String = getInstance("RSA/ECB/NoPadding")
}
