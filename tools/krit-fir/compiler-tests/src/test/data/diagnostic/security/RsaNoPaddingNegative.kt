// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: padded RSA, non-RSA NoPadding, malformed transformations,
// non-literal arguments, aliased imports, and local getInstance lookalikes
// must NOT trigger RsaNoPadding.
package test

import javax.crypto.Cipher
import javax.crypto.Cipher as JCipher

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

    // Go requires the receiver to be spelled `Cipher` or `javax.crypto.Cipher`.
    fun aliased(): Cipher = JCipher.getInstance("RSA/ECB/NoPadding")

    // Local lookalikes: same method name, different owner.
    fun lookalikeObject(): String = KeyCipher.getInstance("RSA/ECB/NoPadding")

    fun getInstance(transformation: String): String = transformation

    fun lookalikeMember(): String = getInstance("RSA/ECB/NoPadding")
}
