// RENDER_DIAGNOSTICS_FULL_TEXT
// Go's same-file lookalike guard: a class named `Cipher` anywhere in the file
// makes a bare `Cipher` receiver ambiguous, so only the fully qualified call
// triggers RsaNoPadding here.
package test

import javax.crypto.Cipher

class Holder {
    class Cipher
}

class Crypto {
    fun bare(): Cipher = Cipher.getInstance("RSA/ECB/NoPadding")

    fun fullyQualified(): Cipher = <!RsaNoPadding!>javax.crypto.Cipher.getInstance("RSA/ECB/NoPadding")<!>
}
