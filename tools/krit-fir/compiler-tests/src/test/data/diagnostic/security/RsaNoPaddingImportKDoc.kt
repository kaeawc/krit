// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Positive: a KDoc right after the last import, the Cipher star import, does
// not change what `Cipher` resolves to, so the call is a real finding. Go
// agrees: tree-sitter attaches the KDoc to the import_header, and Go reads the
// import's identifier path and wildcard, not the header text; FIR reads the
// resolved import.
package test

import javax.crypto.*

/** Crypto helper. */
class Crypto {
    fun f(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>
}
