// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: a class named `Cipher` in the same package wins over the
// javax.crypto star import, so the bare call is not
// javax.crypto.Cipher.getInstance, while the fully qualified call still is.
package test

import javax.crypto.*

class Cipher {
    companion object {
        fun getInstance(transformation: String): String = transformation
    }
}

class Keys {
    fun samePackage(): String = Cipher.getInstance("RSA/ECB/NoPadding")

    fun fullyQualified(): javax.crypto.Cipher = <!RsaNoPadding!>javax.crypto.Cipher.getInstance("RSA/ECB/NoPadding")<!>
}
