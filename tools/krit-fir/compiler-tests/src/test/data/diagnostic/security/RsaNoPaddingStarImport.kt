// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: a star import of javax.crypto backs a bare `Cipher` receiver, and a
// companion object named `Cipher` in another class does not shadow it.
package test

import javax.crypto.*

class Keys {
    companion object Cipher
}

class Crypto {
    fun starImported(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>
}
