// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate improvement: a KDoc right after the last import, the Cipher star
// import, does not change what `Cipher` resolves to, so the call is a real
// finding. Go misses it because tree-sitter attaches the KDoc to the
// import_header and Go compares the whole header text; FIR is correct because
// it reads the resolved import.
package test

import javax.crypto.*

/** Crypto helper. */
class Crypto {
    fun f(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>
}
