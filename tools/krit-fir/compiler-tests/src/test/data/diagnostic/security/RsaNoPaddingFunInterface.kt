// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24
// A nested `fun interface Cipher` does not shadow the imported javax.crypto.Cipher
// at top level, so the bare call is a real finding. Go reports it too: a
// `Cipher` nested in a class that does not enclose the call does not shadow the
// import (and tree-sitter cannot parse `fun interface` in the first place).
package test

import javax.crypto.Cipher

class Outer {
    fun interface Cipher {
        fun g()
    }
}

class Split {
    fun
    interface Cipher {
        fun g()
    }
}

fun f(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>
