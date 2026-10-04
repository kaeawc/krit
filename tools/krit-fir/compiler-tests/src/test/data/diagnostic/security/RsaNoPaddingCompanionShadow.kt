// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: a companion object named `Cipher` in the calling class wins over the
// javax.crypto star import, so the call is not javax.crypto.Cipher.getInstance.
package test

import javax.crypto.*

class Crypto {
    companion object Cipher {
        fun getInstance(transformation: String): String = transformation
    }

    // Go agrees: a companion object named `Cipher` in a class that encloses the
    // call shadows the import, as it does in FIR's resolution.
    fun companion(): String = Cipher.getInstance("RSA/ECB/NoPadding")
}
