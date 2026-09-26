// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: other spellings of javax.crypto.Cipher.getInstance.
// Go misses every call below because it only accepts a receiver spelled
// `Cipher` or `javax.crypto.Cipher`: an import alias, a typealias, a statically
// imported getInstance (plain and aliased), a parenthesized receiver, and a
// backticked one. FIR is correct to report them because each call resolves to
// javax.crypto.Cipher.getInstance with an RSA NoPadding transformation.
package test

import javax.crypto.Cipher
import javax.crypto.Cipher as JCipher
import javax.crypto.Cipher.getInstance
import javax.crypto.Cipher.getInstance as cipherFor

typealias AliasedCipher = javax.crypto.Cipher

class Crypto {
    fun importAlias(): Cipher = <!RsaNoPadding!>JCipher.getInstance("RSA/ECB/NoPadding")<!>

    fun typeAlias(): Cipher = <!RsaNoPadding!>AliasedCipher.getInstance("RSA/ECB/NoPadding")<!>

    fun staticImport(): Cipher = <!RsaNoPadding!>getInstance("RSA/NONE/NoPadding")<!>

    fun aliasedStaticImport(): Cipher = <!RsaNoPadding!>cipherFor("RSA/ECB/NoPadding")<!>

    fun parenthesizedReceiver(): Cipher = <!RsaNoPadding!>(Cipher).getInstance("RSA/ECB/NoPadding")<!>

    fun backtickedReceiver(): Cipher = <!RsaNoPadding!>`Cipher`.getInstance("RSA/ECB/NoPadding")<!>

    fun padded(): Cipher = JCipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding")

    fun staticPadded(): Cipher = getInstance("RSA/ECB/PKCS1Padding")
}
