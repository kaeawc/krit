// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: javax.crypto.Cipher.getInstance with an RSA/<mode>/NoPadding
// transformation literal, through an imported `Cipher` or the fully qualified
// name, triggers RsaNoPadding on the line where the call expression starts.
package test

import java.security.Provider
import javax.crypto.Cipher

class Crypto {
    fun imported(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>

    fun noneMode(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/NONE/NoPadding")<!>

    fun fullyQualified(): Cipher = <!RsaNoPadding!>javax.crypto.Cipher.getInstance("RSA/ECB/NoPadding")<!>

    fun caseAndWhitespaceInsensitive(): Cipher = <!RsaNoPadding!>Cipher.getInstance("  rsa/ecb/nopadding ")<!>

    fun rawString(): Cipher = <!RsaNoPadding!>Cipher.getInstance("""RSA/ECB/NoPadding""")<!>

    fun parenthesized(): Cipher = <!RsaNoPadding!>Cipher.getInstance(("RSA/ECB/NoPadding"))<!>

    fun withProviderName(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding", "BC")<!>

    fun withProvider(provider: Provider): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding", provider)<!>

    // Go reports at the start of the call expression: the receiver's line.
    fun multiLine(): Cipher {
        val cipher = <!RsaNoPadding!>Cipher<!>
            .getInstance("RSA/ECB/NoPadding")
        return cipher
    }

    fun chained(): Int = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>.blockSize

    fun inLambda(): () -> Cipher = { <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!> }
}

fun topLevel(): Cipher = <!RsaNoPadding!>Cipher.getInstance("RSA/ECB/NoPadding")<!>
