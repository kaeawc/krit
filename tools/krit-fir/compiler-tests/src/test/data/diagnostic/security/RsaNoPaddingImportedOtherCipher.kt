// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17
// Negative: an explicit import of another `Cipher` wins over the javax.crypto
// star import, so the bare call is not javax.crypto.Cipher.getInstance.
package test

import javax.crypto.*
import test.KeyCipher as Cipher

object KeyCipher {
    fun getInstance(transformation: String): String = transformation
}

class Crypto {
    // Go reports this because it sees the star import and no declaration named
    // Cipher; FIR is correct because `Cipher` resolves to KeyCipher.
    fun imported(): String = Cipher.getInstance("RSA/ECB/NoPadding")
}
