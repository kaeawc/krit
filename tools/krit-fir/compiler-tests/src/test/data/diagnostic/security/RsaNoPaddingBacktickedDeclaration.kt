// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19
// Nested declarations named `Cipher` in backticks do not shadow the imported
// javax.crypto.Cipher outside their owners, so the bare call is a real finding.
// Go reports it too: its same-file guard strips the backticks, and a `Cipher`
// nested in a class that does not enclose the call does not shadow the import.
package test

import javax.crypto.Cipher

class Holder {
    class `Cipher`
}

class Outer {
    object `Cipher`
}

fun f(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>
