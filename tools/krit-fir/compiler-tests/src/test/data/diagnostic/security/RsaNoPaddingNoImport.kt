// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
// A fully qualified javax.crypto.Cipher receiver needs no import. Go requires
// the receiver text to be exactly `Cipher` or `javax.crypto.Cipher`, so a fully
// qualified receiver split across lines is not reported by either side.
package test

class Crypto {
    fun fullyQualified(): javax.crypto.Cipher = <!RsaNoPadding!>javax.crypto.Cipher.getInstance("RSA/ECB/NoPadding")<!>

    fun padded(): javax.crypto.Cipher = javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding")

    fun splitReceiver(): javax.crypto.Cipher = javax.crypto
        .Cipher.getInstance("RSA/ECB/NoPadding")
}
