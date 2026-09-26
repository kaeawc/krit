// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// A fully qualified javax.crypto.Cipher receiver needs no import.
// Deliberate improvement: Go misses the receiver split across lines because it
// requires the receiver text to be exactly `Cipher` or `javax.crypto.Cipher`;
// FIR is correct to report it because the call still resolves to
// javax.crypto.Cipher.getInstance.
package test

class Crypto {
    fun fullyQualified(): javax.crypto.Cipher = <!RsaNoPadding!>javax.crypto.Cipher.getInstance("RSA/ECB/NoPadding")<!>

    fun padded(): javax.crypto.Cipher = javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding")

    fun splitReceiver(): javax.crypto.Cipher = <!RsaNoPadding!>javax.crypto<!>
        .Cipher.getInstance("RSA/ECB/NoPadding")
}
