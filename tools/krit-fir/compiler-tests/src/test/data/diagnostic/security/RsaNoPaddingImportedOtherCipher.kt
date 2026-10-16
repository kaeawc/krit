// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: an explicit import of another `Cipher` wins over the javax.crypto
// star import, so the bare call is not javax.crypto.Cipher.getInstance.
package test

import javax.crypto.*
import test.KeyCipher as Cipher

object KeyCipher {
    fun getInstance(transformation: String): String = transformation
}

class Crypto {
    // Go agrees: an explicit import that binds another type to `Cipher` wins
    // over the star import; in FIR `Cipher` resolves to KeyCipher.
    fun imported(): String = Cipher.getInstance("RSA/ECB/NoPadding")
}
