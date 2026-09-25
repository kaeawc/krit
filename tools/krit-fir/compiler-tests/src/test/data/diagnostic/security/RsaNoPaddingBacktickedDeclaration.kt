// RENDER_DIAGNOSTICS_FULL_TEXT
// Nested declarations named `Cipher` in backticks do not shadow the imported
// javax.crypto.Cipher outside their owners, so the bare call is a real finding.
// Go reports it too, because its same-file guard compares the backticked source
// text and does not see a declaration named Cipher.
package test

import javax.crypto.Cipher

class Holder {
    class `Cipher`
}

class Outer {
    object `Cipher`
}

fun f(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>
