// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 17
// Positive: a comment on the Cipher import line and a KDoc after the last
// import do not change what `Cipher` resolves to, so the calls are real
// findings. Go agrees: tree-sitter attaches the trailing comment to the
// import_header, and Go reads the import's identifier path, not the header
// text; FIR reads the resolved import.
package test

import java.security.Provider
import javax.crypto.Cipher // RSA helper

/** Crypto helper. */
class Crypto {
    fun f(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>

    fun g(provider: Provider): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/NONE/NoPadding", provider)<!>
}
