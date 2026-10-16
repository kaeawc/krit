// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27
// Negative: a bare `Cipher` that resolves to something other than
// javax.crypto.Cipher, and a raw string whose value is not a valid
// transformation, must NOT trigger RsaNoPadding.
package test

import javax.crypto.Cipher

object KeyCipher {
    fun getInstance(transformation: String): String = transformation
}

class Crypto {
    // Go agrees: a parameter or an earlier local named `Cipher` in scope at the
    // call shadows the imported class, as it does in FIR's resolution.
    fun parameter(Cipher: KeyCipher): String = Cipher.getInstance("RSA/ECB/NoPadding")

    fun localVal(): String {
        val Cipher = KeyCipher
        return Cipher.getInstance("RSA/ECB/NoPadding")
    }

    // Go reports this because tree-sitter folds every trailing quote into the raw
    // string's end token; FIR is correct because the value is
    // `RSA/ECB/NoPadding"`, which is not a NoPadding transformation.
    fun rawStringExtraQuote(): Cipher = Cipher.getInstance("""RSA/ECB/NoPadding"""")
}
