// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 23
// Positive: declarations named `Cipher` nested in other classes do not shadow
// the imported javax.crypto.Cipher in `Crypto`, so the bare call is a real
// finding. Go agrees: its same-file guard only counts a `Cipher` declared at
// top level or in a class that encloses the call; FIR reads the resolved
// receiver.
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
