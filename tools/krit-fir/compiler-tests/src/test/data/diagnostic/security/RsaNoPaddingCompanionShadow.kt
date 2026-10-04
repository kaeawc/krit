// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Negative: a companion object named `Cipher` in the calling class wins over the
// javax.crypto star import, so the call is not javax.crypto.Cipher.getInstance.
package test

import javax.crypto.*

class Crypto {
    companion object Cipher {
        fun getInstance(transformation: String): String = transformation
    }

    // Go reports this because its same-file guard skips companion objects; FIR
    // is correct because `Cipher` resolves to the companion.
    fun companion(): String = Cipher.getInstance("RSA/ECB/NoPadding")
}
