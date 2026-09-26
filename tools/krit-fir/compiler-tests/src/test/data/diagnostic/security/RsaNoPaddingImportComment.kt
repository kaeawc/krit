// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvement: a comment on the Cipher import line and a KDoc after
// the last import do not change what `Cipher` resolves to, so the calls are
// real findings. Go misses them because tree-sitter attaches the trailing
// comment to the import_header and Go compares the whole header text; FIR is
// correct because it reads the resolved import.
package test

import java.security.Provider
import javax.crypto.Cipher // RSA helper

/** Crypto helper. */
class Crypto {
    fun f(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>

    fun g(provider: Provider): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/NONE/NoPadding", provider)<!>
}
