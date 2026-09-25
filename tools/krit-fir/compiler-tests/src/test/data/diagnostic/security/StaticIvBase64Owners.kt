// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: a decode on any class named Base64, not only the platform
// decoders. Go reports each call because the argument text holds
// `Base64.decode(` and a quote. The decoders below really decode the literal,
// so each IV is static and built from literal bytes. FIR matches the owner's
// class name, as Go does, and still requires the decoded data to be a literal.
package test

import javax.crypto.spec.IvParameterSpec

object Base64 {
    fun decode(text: String, flags: Int): ByteArray = java.util.Base64.getDecoder().decode(text.reversed())
}

object MyCodec {
    object Base64 {
        fun decode(text: String): ByteArray = java.util.Base64.getDecoder().decode(text)
    }
}

object Codec {
    fun decode(text: String): ByteArray = java.util.Base64.getDecoder().decode(text)
}

class Crypto(private val stored: String) {
    fun sameFile() = <!StaticIv!>IvParameterSpec(Base64.decode("AAAAAAAAAAAAAAAAAAAAAA==", 0))<!>

    fun nested() = <!StaticIv!>IvParameterSpec(MyCodec.Base64.decode("AAAAAAAAAAAAAAAAAAAAAA=="))<!>

    // Neither reports: the decoded data is not a literal.
    fun stored() = IvParameterSpec(MyCodec.Base64.decode(stored))

    // Neither reports: the owner is not named Base64.
    fun otherCodec() = IvParameterSpec(Codec.decode("AAAAAAAAAAAAAAAAAAAAAA=="))
}
