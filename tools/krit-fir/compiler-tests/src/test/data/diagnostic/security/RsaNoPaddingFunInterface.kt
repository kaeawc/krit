// RENDER_DIAGNOSTICS_FULL_TEXT
// A nested `fun interface Cipher` does not shadow the imported javax.crypto.Cipher
// at top level, so the bare call is a real finding. Go reports it too, because
// tree-sitter cannot parse `fun interface` and its same-file Cipher guard never
// sees the declaration.
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
