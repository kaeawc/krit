// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate improvement: declarations named `Cipher` nested in other classes do
// not shadow the imported javax.crypto.Cipher in `Crypto`, so the bare call is a
// real finding. Go misses it because its same-file guard gives up on a bare
// `Cipher` when any class, object, or type alias in the file is named Cipher;
// FIR is correct because it reads the resolved receiver.
package test

import javax.crypto.Cipher

class Holder {
    class Cipher
}

class Other {
    object Cipher
}

class Crypto {
    fun bare(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>

    fun fullyQualified(): Cipher = <!RsaNoPadding!>javax.crypto.Cipher.getInstance("RSA/ECB/NoPadding")<!>
}

// Inside the owner, the nested class shadows the import, as in Go.
class Owner {
    class Cipher {
        companion object {
            fun getInstance(transformation: String): String = transformation
        }
    }

    fun nested(): String = Cipher.getInstance("RSA/ECB/NoPadding")
}
