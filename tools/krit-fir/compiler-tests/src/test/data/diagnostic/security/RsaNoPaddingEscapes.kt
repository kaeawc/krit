// RENDER_DIAGNOSTICS_FULL_TEXT
// Go reads the transformation from the literal's source text with escape
// sequences left undecoded: a trailing escaped newline or tab is not trimmed
// whitespace, while an escaped backslash inside the mode still leaves three
// segments. A raw string has no escapes, so its surrounding newlines trim.
package test

import javax.crypto.Cipher

class Crypto {
    fun escapedNewline(): Cipher = Cipher.getInstance("RSA/ECB/NoPadding\n")

    fun escapedTab(): Cipher = Cipher.getInstance("RSA/ECB/NoPadding\t")

    fun escapedBackslashInMode(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB\\/NoPadding")<!>

    fun multiLineRaw(): Cipher = <!RsaNoPadding!>Cipher.getInstance(<!>
        """
        RSA/ECB/NoPadding
        """
    )
}
